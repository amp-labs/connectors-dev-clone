package providers

const Clarify Provider = "clarify"

func init() {
	SetInfo(Clarify, ProviderInfo{
		DisplayName: "Clarify",
		AuthType:    ApiKey,
		// Every API path is versioned and workspace-scoped: /v1/workspaces/{workspace}/...
		// https://developer.clarify.ai/docs#base-url
		BaseURL: "https://api.clarify.ai",
		ApiKeyOpts: &ApiKeyOpts{
			AttachmentType: Header,
			Header: &ApiKeyOptsHeader{
				Name:        "Authorization",
				ValuePrefix: "api-key ",
			},
			DocsURL: "https://developer.clarify.ai/docs/getting-started/authentication#api-keys",
		},
		Metadata: &ProviderMetadata{
			Input: []MetadataItemInput{
				{
					Name:        "workspace",
					DisplayName: "Workspace Slug",
					DocsURL:     "https://developer.clarify.ai/docs/getting-started/key-concepts#workspace",
					Prompt:      "The workspace slug appears in your Clarify login URL and under the user avatar menu.",
				},
			},
		},
		Support: Support{
			BulkWrite: BulkWriteSupport{
				Insert: false,
				Update: false,
				Upsert: false,
				Delete: false,
			},
			Proxy:     false,
			Read:      true,
			Subscribe: false,
			Write:     false,
		},
	})
}
