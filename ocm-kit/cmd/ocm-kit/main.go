package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci/compref"
	ctfv1 "ocm.software/open-component-model/bindings/go/oci/spec/repository/v1/ctf"

	"github.com/open-component-model/community/ocm-kit/compver"
	"github.com/open-component-model/community/ocm-kit/helmvalues"
)

// version is the ocm-kit release version, injected at build time via -ldflags
// "-X main.version=...". It defaults to "dev" for local builds.
var version = "dev"

func main() {
	var opts options

	rootCmd := &cobra.Command{
		Use:     "ocm-kit <component-version-ref>|<ctf-path>",
		Version: version,
		Short:   "OCM Kit - Render Helm values templates from OCM components",
		Long: `OCM Kit renders Helm values templates embedded in OCM (Open Component Model) components.

It takes a component version reference and renders the first Helm values template for a specified chart.

A local CTF (directory, .tar, .tgz or .tar.gz) is read with ctf::<path>//<component>:<version>, or with the bare path when it holds exactly one component version.
Component-local images from a CTF render with the placeholder host REGISTRY, because a CTF has no registry host.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			componentVersionRef := args[0]

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if strings.HasPrefix(componentVersionRef, ctfPrefix) || !strings.Contains(componentVersionRef, "//") {
				return opts.renderCTF(ctx, componentVersionRef)
			}

			cvr, err := compver.SplitRef(componentVersionRef)
			if err != nil {
				return fmt.Errorf("failed to split component version reference: %w", err)
			}

			client, err := helmvalues.NewAuthClient(ctx, "")
			if err != nil {
				return fmt.Errorf("failed to create auth client: %w", err)
			}

			baseURL := cvr.Host + "/" + cvr.Namespace
			repoURL := cvr.Protocol + "://" + baseURL

			repo, err := helmvalues.OpenRepository(repoURL, helmvalues.WithAuthClient(client))
			if err != nil {
				return fmt.Errorf("failed to open repository: %w", err)
			}
			defer func() { _ = repo.Close() }()

			desc, err := repo.GetComponentVersion(ctx, cvr.ComponentName, cvr.Version)
			if err != nil {
				return fmt.Errorf("failed to get component version: %w", err)
			}

			return opts.render(ctx, repo, desc, baseURL)
		},
	}

	rootCmd.Flags().StringVarP(&opts.chartResName, "chart-resource", "r", "", "Name of the Helm chart resource in the component to render a specific helm values template")
	rootCmd.Flags().StringVarP(&opts.localHelmValuesTemplate, "local-helm-values-template", "f", "", "Path to a local Helm values template file (overrides component template)")
	rootCmd.Flags().StringVarP(&opts.pullSecretsFilePath, "pull-secrets-file", "p", "", "Path to a pull secrets JSON file mapping registries to Kubernetes secret names")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ctfPrefix marks a component version reference into a local CTF.
const ctfPrefix = "ctf::"

type options struct {
	chartResName            string
	localHelmValuesTemplate string
	pullSecretsFilePath     string
}

// renderCTF renders from a local CTF, given as ctf::<path>//<component>:<version>
// or as a bare path ([ctf::]<path>) to a CTF holding exactly one component version.
func (o *options) renderCTF(ctx context.Context, arg string) error {
	path := strings.TrimPrefix(arg, ctfPrefix)
	var cv helmvalues.ComponentVersion
	if strings.Contains(arg, "//") {
		ref, err := compref.Parse(arg)
		if err != nil {
			return err
		}
		// Reason: the lookup is by version only and would silently ignore the pin.
		if ref.Digest != "" {
			return errors.New("digest-pinned component references are not supported")
		}
		spec, ok := ref.Repository.(*ctfv1.Repository)
		if !ok {
			return fmt.Errorf("%q is not a CTF reference", arg)
		}
		path = spec.FilePath
		cv = helmvalues.ComponentVersion{Name: ref.Component, Version: ref.Version}
	}

	repo, err := helmvalues.OpenCTF(ctx, path)
	if err != nil {
		return err
	}
	defer func() { _ = repo.Close() }()

	if cv.Name == "" {
		cvs, err := repo.ComponentVersions(ctx)
		if err != nil {
			return err
		}
		switch len(cvs) {
		case 0:
			return fmt.Errorf("no component versions in CTF %q", path)
		case 1:
			cv = cvs[0]
		default:
			lines := make([]string, len(cvs))
			for i, c := range cvs {
				lines[i] = c.Name + ":" + c.Version
			}
			return fmt.Errorf("CTF %q holds several component versions, pass %s%s//<name>:<version> with one of:\n%s",
				path, ctfPrefix, path, strings.Join(lines, "\n"))
		}
	}

	desc, err := repo.GetComponentVersion(ctx, cv.Name, cv.Version)
	if err != nil {
		return fmt.Errorf("getting component version %s:%s: %w", cv.Name, cv.Version, err)
	}
	return o.render(ctx, repo, desc, helmvalues.CTFPlaceholderHost)
}

// render renders the selected Helm values template of desc, resolving
// component-local image references against baseURL.
func (o *options) render(ctx context.Context, repo helmvalues.Repository, desc *descriptor.Descriptor, baseURL string) error {
	var (
		template *helmvalues.HelmValuesTemplate
		err      error
	)

	// Use local file if provided, otherwise fetch from component
	if o.localHelmValuesTemplate != "" {
		content, err := os.ReadFile(o.localHelmValuesTemplate)
		if err != nil {
			return fmt.Errorf("failed to read local helm values template: %w", err)
		}
		template = &helmvalues.HelmValuesTemplate{
			ResourceName:    "local-file",
			ResourceVersion: "0.0.0",
			TemplateContent: string(content),
		}
	} else if o.chartResName != "" {
		template, err = helmvalues.GetHelmValuesTemplate(ctx, repo, desc, o.chartResName)
		if err != nil {
			return fmt.Errorf("failed to get helm values template: %w", err)
		}
	} else {
		template, err = helmvalues.GetFirstHelmValuesTemplate(ctx, repo, desc)
		if err != nil {
			return fmt.Errorf("failed to get helm values template: %w", err)
		}
	}

	input, err := helmvalues.GetRenderingInput(desc, baseURL)
	if err != nil {
		return fmt.Errorf("failed to build rendering input: %w", err)
	}

	if o.pullSecretsFilePath != "" {
		ps, err := helmvalues.ParsePullSecretsFile(o.pullSecretsFilePath)
		if err != nil {
			return fmt.Errorf("failed to parse pull secrets file: %w", err)
		}
		input.PullSecrets = ps
	}

	output, err := helmvalues.Render(template, input)
	if err != nil {
		return fmt.Errorf("failed to render helm values template: %w", err)
	}

	fmt.Println(output)
	return nil
}
