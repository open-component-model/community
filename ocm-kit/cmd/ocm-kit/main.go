package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/open-component-model/community/ocm-kit/compver"
	"github.com/open-component-model/community/ocm-kit/helmvalues"
)

// version is the ocm-kit release version, injected at build time via -ldflags
// "-X main.version=...". It defaults to "dev" for local builds.
var version = "dev"

func main() {
	var (
		chartResName            string
		localHelmValuesTemplate string
		pullSecretsFilePath     string
	)

	rootCmd := &cobra.Command{
		Use:     "ocm-kit <component-version-ref>",
		Version: version,
		Short:   "OCM Kit - Render Helm values templates from OCM components",
		Long: `OCM Kit renders Helm values templates embedded in OCM (Open Component Model) components.

It takes a component version reference and renders the first Helm values template for a specified chart.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			componentVersionRef := args[0]

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

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

			var template *helmvalues.HelmValuesTemplate

			// Use local file if provided, otherwise fetch from component
			if localHelmValuesTemplate != "" {
				content, err := os.ReadFile(localHelmValuesTemplate)
				if err != nil {
					return fmt.Errorf("failed to read local helm values template: %w", err)
				}
				template = &helmvalues.HelmValuesTemplate{
					ResourceName:    "local-file",
					ResourceVersion: "0.0.0",
					TemplateContent: string(content),
				}
			} else if chartResName != "" {
				template, err = helmvalues.GetHelmValuesTemplate(ctx, repo, desc, chartResName)
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

			if pullSecretsFilePath != "" {
				ps, err := helmvalues.ParsePullSecretsFile(pullSecretsFilePath)
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
		},
	}

	rootCmd.Flags().StringVarP(&chartResName, "chart-resource", "r", "", "Name of the Helm chart resource in the component to render a specific helm values template")
	rootCmd.Flags().StringVarP(&localHelmValuesTemplate, "local-helm-values-template", "f", "", "Path to a local Helm values template file (overrides component template)")
	rootCmd.Flags().StringVarP(&pullSecretsFilePath, "pull-secrets-file", "p", "", "Path to a pull secrets JSON file mapping registries to Kubernetes secret names")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
