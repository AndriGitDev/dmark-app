package ingest

import (
	"context"
	"testing"
)

func FuzzParseAggregatePayload(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(sampleReport),
		[]byte(`<?xml version="1.0"?><feedback></feedback>`),
		[]byte(`<feedback><report_metadata><report_id>x</report_id></report_metadata></feedback>`),
		[]byte(`not xml`),
		[]byte(``),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		_, _ = ParseAggregatePayload(context.Background(), "fuzz.xml", payload, 64*1024)
	})
}
