package identifiers

import (
	"context"
	"testing"
)

func TestParseNodeID(t *testing.T) {
	tests := []struct {
		name     string
		nodeID   string
		wantName string
		wantFQDN string
		wantIP   string
		wantErr  bool
	}{
		{
			name:     "Valid Node ID",
			nodeID:   "node1=#=#=fqdn.example.com=#=#=192.168.1.1",
			wantName: "node1",
			wantFQDN: "fqdn.example.com",
			wantIP:   "192.168.1.1",
			wantErr:  false,
		},
		{
			name:    "Invalid Node ID - Missing Sections",
			nodeID:  "node1=#=#=fqdn.example.com",
			wantErr: true,
		},
		{
			name:    "Invalid Node ID - Empty String",
			nodeID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			gotName, gotFQDN, gotIP, err := ParseNodeID(ctx, tt.nodeID)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNodeID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotName != tt.wantName {
				t.Errorf("ParseNodeID() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotFQDN != tt.wantFQDN {
				t.Errorf("ParseNodeID() gotFQDN = %v, want %v", gotFQDN, tt.wantFQDN)
			}
			if gotIP != tt.wantIP {
				t.Errorf("ParseNodeID() gotIP = %v, want %v", gotIP, tt.wantIP)
			}
		})
	}
}
