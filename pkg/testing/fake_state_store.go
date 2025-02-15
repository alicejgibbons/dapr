/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"

	state "github.com/dapr/components-contrib/state"
)

type FakeStateStoreItem struct {
	data []byte
	etag *string
}

type FakeStateStore struct {
	Items map[string]*FakeStateStoreItem

	lock sync.RWMutex
}

func NewFakeStateStore() *FakeStateStore {
	return &FakeStateStore{
		Items: map[string]*FakeStateStoreItem{},
	}
}

func (f *FakeStateStore) NewItem(data []byte) *FakeStateStoreItem {
	etag, _ := uuid.NewRandom()
	etagString := etag.String()
	return &FakeStateStoreItem{
		data: data,
		etag: &etagString,
	}
}

func (f *FakeStateStore) Init(ctx context.Context, metadata state.Metadata) error {
	return nil
}

func (f *FakeStateStore) Ping() error {
	return nil
}

func (f *FakeStateStore) Features() []state.Feature {
	return []state.Feature{state.FeatureETag, state.FeatureTransactional}
}

func (f *FakeStateStore) Delete(ctx context.Context, req *state.DeleteRequest) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	delete(f.Items, req.Key)

	return nil
}

func (f *FakeStateStore) BulkDelete(ctx context.Context, req []state.DeleteRequest) error {
	return nil
}

func (f *FakeStateStore) Get(ctx context.Context, req *state.GetRequest) (*state.GetResponse, error) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	item := f.Items[req.Key]

	if item == nil {
		return &state.GetResponse{Data: nil, ETag: nil}, nil
	}

	return &state.GetResponse{Data: item.data, ETag: item.etag}, nil
}

func (f *FakeStateStore) BulkGet(ctx context.Context, req []state.GetRequest, opts state.BulkGetOpts) ([]state.BulkGetResponse, error) {
	res := []state.BulkGetResponse{}
	for _, oneRequest := range req {
		oneResponse, err := f.Get(ctx, &state.GetRequest{
			Key:      oneRequest.Key,
			Metadata: oneRequest.Metadata,
			Options:  oneRequest.Options,
		})
		if err != nil {
			return nil, err
		}

		res = append(res, state.BulkGetResponse{
			Key:  oneRequest.Key,
			Data: oneResponse.Data,
			ETag: oneResponse.ETag,
		})
	}

	return res, nil
}

func (f *FakeStateStore) Set(ctx context.Context, req *state.SetRequest) error {
	b, _ := marshal(&req.Value)
	f.lock.Lock()
	defer f.lock.Unlock()
	f.Items[req.Key] = f.NewItem(b)

	return nil
}

func (f *FakeStateStore) GetComponentMetadata() map[string]string {
	return map[string]string{}
}

func (f *FakeStateStore) BulkSet(ctx context.Context, req []state.SetRequest) error {
	return nil
}

func (f *FakeStateStore) Multi(ctx context.Context, request *state.TransactionalStateRequest) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	// First we check all eTags
	for _, o := range request.Operations {
		var eTag *string
		key := ""
		switch req := o.(type) {
		case state.SetRequest:
			key = req.Key
			eTag = req.ETag
		case state.DeleteRequest:
			key = req.Key
			eTag = req.ETag
		}
		item := f.Items[key]
		if eTag != nil && item != nil {
			if *eTag != *item.etag {
				return fmt.Errorf("etag does not match for key %v", key)
			}
		}
		if eTag != nil && item == nil {
			return fmt.Errorf("etag does not match for key not found %v", key)
		}
	}

	// Now we can perform the operation.
	for _, o := range request.Operations {
		switch req := o.(type) {
		case state.SetRequest:
			b, _ := marshal(req.Value)
			f.Items[req.Key] = f.NewItem(b)
		case state.DeleteRequest:
			delete(f.Items, req.Key)
		}
	}

	return nil
}

// Adapted from https://github.com/dapr/components-contrib/blob/a4b27ae49b7c99820c6e921d3891f03334692714/state/utils/utils.go#L16
func marshal(val interface{}) ([]byte, error) {
	var err error = nil
	bt, ok := val.([]byte)
	if !ok {
		bt, err = json.Marshal(val)
	}

	return bt, err
}
