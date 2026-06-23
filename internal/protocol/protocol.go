package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"commitment-ledger/internal/cid"
)

const (
	CommitmentPromise         = "commitment-promise-v1"
	CommitmentEvidenceV1      = "commitment-evidence-v1"
	CommitmentEvidence        = "commitment-evidence-v2"
	CommitmentAssessmentV1    = "commitment-assessment-v1"
	CommitmentAssessment      = "commitment-assessment-v2"
	ImplementationConformance = "implementation-conformance-v1"
)

type Spec struct {
	Name   string
	Path   string
	Bytes  []byte
	PCID   string
	DocCID string
}

type Registry struct {
	ByName map[string]Spec
}

func (r Registry) Specs() []Spec {
	out := make([]Spec, 0, len(r.ByName))
	for _, spec := range r.ByName {
		out = append(out, spec)
	}
	return out
}

func Load(root string) (Registry, error) {
	names := []string{
		CommitmentPromise,
		CommitmentEvidenceV1,
		CommitmentEvidence,
		CommitmentAssessmentV1,
		CommitmentAssessment,
		ImplementationConformance,
	}
	reg := Registry{ByName: map[string]Spec{}}
	for _, name := range names {
		path := filepath.Join(root, "docs", "protocols", name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			return Registry{}, fmt.Errorf("read protocol spec %q: %w", path, err)
		}
		id := cid.Sum(data)
		reg.ByName[name] = Spec{
			Name:   name,
			Path:   path,
			Bytes:  data,
			PCID:   id,
			DocCID: id,
		}
	}
	return reg, nil
}

func (r Registry) MustPCID(name string) string {
	spec, ok := r.ByName[name]
	if !ok {
		panic("unknown protocol: " + name)
	}
	return spec.PCID
}

func MarshalPayload(v any) ([]byte, error) {
	return json.Marshal(v)
}
