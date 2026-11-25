package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"

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
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			opened, err := compver.Open(ctx, args[0])
			if err != nil {
				return err
			}
			defer func() { _ = opened.Close() }()
			return opts.render(ctx, opened.Repository, opened.Descriptor, opened.BaseURL)
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

type options struct {
	chartResName            string
	localHelmValuesTemplate string
	pullSecretsFilePath     string
}

// render renders the selected Helm values template of desc, resolving
// component-local image references against baseURL.
func (o *options) render(ctx context.Context, repo compver.Repository, desc *descriptor.Descriptor, baseURL string) error {
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
