package messaging

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestProofSignatureIntactRequiresTheDeclarationForTheSameChild(t *testing.T) {
	t.Parallel()

	const signatureID int64 = 41
	const studentID int64 = 53
	otherStudentID := studentID + 1
	submission := &DeclarationSubmission{
		StudentID: studentID, SignatureID: testpkg.Int64Ptr(signatureID), ContentHash: "content-hash",
	}
	proof := &DeclarationProof{Signatures: map[int64]*DeclarationSignature{
		signatureID: {
			IsDeclaration: true, StudentID: testpkg.Int64Ptr(studentID), DocumentSHA256: submission.ContentHash, IntegrityOK: true,
		},
	}}

	if !proofSignatureIntact(proof, submission) {
		t.Fatal("expected matching declaration signature to be intact")
	}
	proof.Signatures[signatureID].IsDeclaration = false
	if proofSignatureIntact(proof, submission) {
		t.Error("non-declaration signature must not be intact")
	}
	proof.Signatures[signatureID].IsDeclaration = true
	proof.Signatures[signatureID].StudentID = testpkg.Int64Ptr(otherStudentID)
	if proofSignatureIntact(proof, submission) {
		t.Error("signature for another child must not be intact")
	}
	proof.Signatures[signatureID] = nil
	if proofSignatureIntact(proof, submission) {
		t.Error("missing signature must not be intact")
	}
}
