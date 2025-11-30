package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
)

type verifyTrustedMaterial struct {
	root.TrustedMaterial
	keyTrustedMaterial root.TrustedMaterial
}

func (v *verifyTrustedMaterial) PublicKeyVerifier(hint string) (root.TimeConstrainedVerifier, error) {
	return v.keyTrustedMaterial.PublicKeyVerifier(hint)
}

type options struct {
	gitlabClient    func() (*gitlab.Client, error)
	defaultHostname string
	io              *iostreams.IOStreams

	project  string
	filename string
}

func NewCmdVerify(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		gitlabClient:    f.GitLabClient,
		defaultHostname: f.DefaultHostname(),
		io:              f.IO(),
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
			opts.filename = args[0]

			err := opts.run()
			if err != nil {
				return err
			}

			return nil
		},
	}

	attestationVerifyCmd.Flags().StringVarP(&opts.project, "project", "p", "", "Project id or path")
	attestationVerifyCmd.MarkFlagRequired("project")

	return attestationVerifyCmd
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	project, err := api.GetProject(client, o.project)
	if err != nil {
		return err
	}

	subjectDigest, err := o.sha256(o.filename)
	if err != nil {
		return err
	}

	provenance, err := o.retrieveProvenanceMetadata(client, subjectDigest)
	if err != nil {
		return err
	}

	bundle, err := o.downloadBundle(client, provenance.IID)
	if err != nil {
		return err
	}

	err = o.verify(client, subjectDigest, project.PathWithNamespace, bundle)
	if err != nil {
		return err
	}

	o.success()

	return nil
}

func (o *options) sha256(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (o *options) retrieveProvenanceMetadata(client *gitlab.Client, subjectDigest string) (*gitlab.Attestation, error) {
	attestations, _, err := client.Attestations.ListAttestations(o.project, subjectDigest)
	if err != nil {
		return nil, err
	}

	for _, attestation := range attestations {
		if attestation.PredicateKind == "provenance" {
			return attestation, nil
		}
	}

	return nil, fmt.Errorf("Unable to find a provenance statement for %s", subjectDigest)
}

func (o *options) downloadBundle(client *gitlab.Client, attestationIID int64) ([]byte, error) {
	provenanceStatement, _, err := client.Attestations.DownloadAttestation(o.project, attestationIID)
	if err != nil {
		return nil, err
	}

	return provenanceStatement, nil
}

func (o *options) verify(client *gitlab.Client, subjectDigest string, repoPath string, bundleBytes []byte) error {
	opts := tuf.DefaultOptions()
	tufClient, err := tuf.New(opts)
	if err != nil {
		return err
	}

	trustedMaterial, err := root.GetTrustedRoot(tufClient)
	if err != nil {
		return err
	}

	sev, err := verify.NewVerifier(trustedMaterial, verify.WithSignedCertificateTimestamps(1), verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1))
	if err != nil {
		return err
	}

	digest, err := hex.DecodeString(subjectDigest)
	if err != nil {
		return err
	}

	expectedIssuer := fmt.Sprintf("https://%s", o.defaultHostname)
	expectedSanRegex := fmt.Sprintf("^https://%s/%s/", o.defaultHostname, repoPath)
	certID, err := verify.NewShortCertificateIdentity(expectedIssuer, "", "", expectedSanRegex)
	if err != nil {
		return err
	}

	var bundle bundle.Bundle
	bundle.Bundle = new(protobundle.Bundle)
	err = bundle.UnmarshalJSON(bundleBytes)
	if err != nil {
		return err
	}

	// If and only if verification is successful, Verify will return a VerificationResult struct whose contents' integrity have been verified.
	_, err = sev.Verify(&bundle, verify.NewPolicy(verify.WithArtifactDigest("sha256", digest), verify.WithCertificateIdentity(certID)))
	if err != nil {
		return err
	}

	return nil
}

func (o *options) success() {
	c := o.io.Color()
	out := o.io.StdOut

	fmt.Fprint(out, c.Green("VERIFIED"))
	fmt.Fprintf(out, " • Artifact provenance successfully verified. Signatures confirm %s was attested by %s\n", o.filename, o.project)
	fmt.Fprintln(out)
}
