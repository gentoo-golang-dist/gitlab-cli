package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
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
	gitlabClient func() (*gitlab.Client, error)
	defaultHostname string

	project string
	filename string
}

func NewCmdVerify(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		gitlabClient: f.GitLabClient,
		defaultHostname: f.DefaultHostname(),
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

	subject_digest, err := o.sha256(o.filename)
	if err != nil {
		return err
	}

	provenance, err := o.retrieveProvenanceMetadata(client, subject_digest)
	if err != nil {
		return err
	}

	bundle, err := o.downloadBundle(client, provenance.IID)
	if err != nil {
		return err
	}

	o.verify(client, subject_digest, o.project, bundle)

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

func (o *options) retrieveProvenanceMetadata(client *gitlab.Client, subject_digest string) (*gitlab.Attestation, error) {
	listAttestationsOptions := &gitlab.ListAttestationsOptions{
		SubjectDigest: subject_digest,
	}

	attestations, _, err := client.Attestations.ListAttestations(o.project, listAttestationsOptions)
	if err != nil {
		return nil, err
	}

	for _, attestation := range attestations {
		if attestation.PredicateKind == "provenance" {
			return attestation, nil
		}
	}

	return nil, fmt.Errorf("Unable to find a provenance statement for %s", subject_digest)
}

func (o *options) downloadBundle(client *gitlab.Client, AttestationIID int) ([]byte, error) {
	downloadAttestationOptions := &gitlab.DownloadAttestationOptions{
		AttestationIID: AttestationIID,
	}

	provenanceStatement, _, err := client.Attestations.DownloadAttestation(o.project, downloadAttestationOptions)
	if err != nil {
		return nil, err
	}

	return provenanceStatement, nil
}

func (o *options) verify(client *gitlab.Client, subject_digest string, repo string, bundleBytes []byte) error {
	opts := tuf.DefaultOptions()
	tufClient, err := tuf.New(opts)
	if err != nil {
		panic(err)
	}

	trustedMaterial, err := root.GetTrustedRoot(tufClient)
	if err != nil {
		panic(err)
	}

	sev, err := verify.NewVerifier(trustedMaterial, verify.WithSignedCertificateTimestamps(1), verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1))
	if err != nil {
		panic(err)
	}

	digest, err := hex.DecodeString(subject_digest)
	if err != nil {
		panic(err)
	}

	expectedIssuer := fmt.Sprintf("https://%s", o.defaultHostname)
	expectedSanRegex := fmt.Sprintf("^https://%s/%s/", o.defaultHostname, o.project)
	certID, err := verify.NewShortCertificateIdentity(expectedIssuer, "", "", expectedSanRegex)
	if err != nil {
		panic(err)
	}

	var bundle bundle.Bundle
	bundle.Bundle = new(protobundle.Bundle)
	err = bundle.UnmarshalJSON(bundleBytes)
	if err != nil {
		panic(err)
	}

	result, err := sev.Verify(&bundle, verify.NewPolicy(verify.WithArtifactDigest("sha256", digest), verify.WithCertificateIdentity(certID)))
	if err != nil {
		panic(err)
	}

	marshaled, err := json.MarshalIndent(result, "", "   ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(marshaled))

	return nil
}
