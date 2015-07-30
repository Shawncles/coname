// Copyright 2014-2015 The Dename Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package proto

import "fmt"

type SignedEntryUpdate_PreserveEncoding struct {
	SignedEntryUpdate
	PreservedEncoding []byte
}

func (m *SignedEntryUpdate_PreserveEncoding) UpdateEncoding() (err error) {
	m.PreservedEncoding, err = m.SignedEntryUpdate.Marshal()
	return err
}

func (m *SignedEntryUpdate_PreserveEncoding) Reset() {
	*m = SignedEntryUpdate_PreserveEncoding{}
}

func (m *SignedEntryUpdate_PreserveEncoding) Size() int {
	return len(m.PreservedEncoding)
}

func (m *SignedEntryUpdate_PreserveEncoding) Marshal() ([]byte, error) {
	size := m.Size()
	data := make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *SignedEntryUpdate_PreserveEncoding) MarshalTo(data []byte) (int, error) {
	return copy(data, m.PreservedEncoding), nil
}

func (m *SignedEntryUpdate_PreserveEncoding) Unmarshal(data []byte) error {
	m.PreservedEncoding = append([]byte{}, data...)
	return m.SignedEntryUpdate.Unmarshal(data)
}

func NewPopulatedSignedEntryUpdate_PreserveEncoding(r randyClient, easy bool) *SignedEntryUpdate_PreserveEncoding {
	this := &SignedEntryUpdate_PreserveEncoding{SignedEntryUpdate: *NewPopulatedSignedEntryUpdate(r, easy)}
	this.UpdateEncoding()
	return this
}

func (this *SignedEntryUpdate_PreserveEncoding) VerboseEqual(that interface{}) error {
	if thatP, ok := that.(*SignedEntryUpdate_PreserveEncoding); ok {
		return this.SignedEntryUpdate.VerboseEqual(&thatP.SignedEntryUpdate)
	}
	if thatP, ok := that.(SignedEntryUpdate_PreserveEncoding); ok {
		return this.SignedEntryUpdate.VerboseEqual(&thatP.SignedEntryUpdate)
	}
	return fmt.Errorf("types don't match: %T != SignedEntryUpdate_PreserveEncoding")
}

func (this *SignedEntryUpdate_PreserveEncoding) Equal(that interface{}) bool {
	if thatP, ok := that.(*SignedEntryUpdate_PreserveEncoding); ok {
		return this.SignedEntryUpdate.Equal(&thatP.SignedEntryUpdate)
	}
	if thatP, ok := that.(SignedEntryUpdate_PreserveEncoding); ok {
		return this.SignedEntryUpdate.Equal(&thatP.SignedEntryUpdate)
	}
	return false
}

func (this *SignedEntryUpdate_PreserveEncoding) GoString() string {
	if this == nil {
		return "nil"
	}
	return `proto.SignedEntryUpdate_PreserveEncoding{SignedEntryUpdate: ` + this.SignedEntryUpdate.GoString() + `, PreservedEncoding: ` + fmt.Sprintf("%#v", this.PreservedEncoding) + `}`
}

func (this *SignedEntryUpdate_PreserveEncoding) String() string {
	if this == nil {
		return "nil"
	}
	return `proto.SignedEntryUpdate_PreserveEncoding{SignedEntryUpdate: ` + this.SignedEntryUpdate.String() + `, PreservedEncoding: ` + fmt.Sprintf("%v", this.PreservedEncoding) + `}`
}