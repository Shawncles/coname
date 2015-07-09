package server

import (
	"fmt"

	"github.com/yahoo/coname/proto"
	"golang.org/x/net/context"
)

func (ks *Keyserver) UpdateProfile(ctx context.Context, req *proto.SignedEntryUpdate) (*proto.LookupProof, error) {
	return nil, fmt.Errorf("UpdateProfile not implemented")
}