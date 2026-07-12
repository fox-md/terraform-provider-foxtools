// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider              = &foxtoolsProvider{}
	_ provider.ProviderWithFunctions = &foxtoolsProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &foxtoolsProvider{
			version: version,
		}
	}
}

// foxtoolsProvider is the provider implementation.
type foxtoolsProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *foxtoolsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "foxtools"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *foxtoolsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "`foxtools` includes below resources:\n" +
			"- `foxtools_file_download` downloads file from remote over http.\n",
	}
}

// Configure prepares an API client for data sources and resources.
func (p *foxtoolsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
}

// DataSources defines the data sources implemented in the provider.
func (p *foxtoolsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// Resources defines the resources implemented in the provider.
func (p *foxtoolsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFileDownloadResource,
	}
}

// Functions defines the functions implemented in the provider.
func (p *foxtoolsProvider) Functions(_ context.Context) []func() function.Function {
	return nil
}
