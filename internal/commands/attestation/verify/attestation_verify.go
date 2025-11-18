package approve

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

type verifyTrustedMaterial struct {
	root.TrustedMaterial
	keyTrustedMaterial root.TrustedMaterial
}

func (v *verifyTrustedMaterial) PublicKeyVerifier(hint string) (root.TimeConstrainedVerifier, error) {
	return v.keyTrustedMaterial.PublicKeyVerifier(hint)
}

type options struct {
	project string

	keyID int
}

func NewCmdVerify(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		// io:           f.IO(),
		// gitlabClient: f.GitLabClient,
		// baseRepo:     f.BaseRepo,
	}

	attestationVerifyCmd := &cobra.Command{
		Use:   "verify <artifact_path>",
		Short: `Verify the provenance of a specific artifact or file`,
		Long:  ``,
		Args: cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			$ glab attestation verify filename.txt --project gilab-org/gitlab
			$ glab attestation verify filename.txt --project 123
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(args[0]);
			fmt.Println(opts.project);
			return nil;

			opts := tuf.DefaultOptions()
			client, err := tuf.New(opts)
			if err != nil {
				panic(err)
			}

			trustedMaterial, err := root.GetTrustedRoot(client)
			if err != nil {
				panic(err)
			}

			sev, err := verify.NewVerifier(trustedMaterial, verify.WithSignedCertificateTimestamps(1), verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1))
			if err != nil {
				panic(err)
			}

			digest, err := hex.DecodeString("76176ffa33808b54602c7c35de5c6e9a4deb96066dba6533f50ac234f4f1f4c6b3527515dc17c06fbe2860030f410eee69ea20079bd3a2c6f3dcf3b329b10751")
			if err != nil {
				panic(err)
			}

			certID, err := verify.NewShortCertificateIdentity("https://token.actions.githubusercontent.com", "", "", "^https://github.com/sigstore/sigstore-js/")
			if err != nil {
				panic(err)
			}

			b, err := bundle.LoadJSONFromPath("../examples/bundle-provenance.json")
			if err != nil {
				panic(err)
			}

			result, err := sev.Verify(b, verify.NewPolicy(verify.WithArtifactDigest("sha512", digest), verify.WithCertificateIdentity(certID)))
			if err != nil {
				panic(err)
			}

			marshaled, err := json.MarshalIndent(result, "", "   ")
			if err != nil {
				panic(err)
			}
			fmt.Println(string(marshaled))

			return nil
		},
	}

	attestationVerifyCmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project id or path")
	attestationVerifyCmd.MarkFlagRequired("project")

	return attestationVerifyCmd
}
