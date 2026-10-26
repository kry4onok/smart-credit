package profile

import (
	"encoding/json"
	"reflect"
	"testing"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	fixtures "github.com/alfabank-hackathon-2026/smart-credit/seed/profile-requirements"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestFixtureRoundTrips(t *testing.T) {
	data, err := fixtures.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range data.Cases {
		t.Run(s.ID, func(t *testing.T) {
			c := FromHTTP(s.Profile)
			raw, err := protojson.Marshal(ToProto(c))
			if err != nil {
				t.Fatal(err)
			}
			wire := &commonpb.CompanySnapshot{}
			if err := protojson.Unmarshal(raw, wire); err != nil {
				t.Fatal(err)
			}
			got := ToHTTP(FromProto(wire))
			wantRaw, _ := json.Marshal(s.Profile)
			gotRaw, _ := json.Marshal(got)
			var wantJSON, gotJSON any
			_ = json.Unmarshal(wantRaw, &wantJSON)
			_ = json.Unmarshal(gotRaw, &gotJSON)
			if !reflect.DeepEqual(wantJSON, gotJSON) {
				t.Fatalf("round trip changed profile\nwant %s\ngot %s", wantRaw, gotRaw)
			}
		})
	}
}
