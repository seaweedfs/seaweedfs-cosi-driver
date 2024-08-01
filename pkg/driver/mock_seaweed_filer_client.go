/*
Copyright 2024 SeaweedFS contributors.

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

package driver

import (
	"context"

	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"google.golang.org/grpc"
)

type mockSeaweedFilerClient struct {
	createEntryFunc                     func(ctx context.Context, in *filer_pb.CreateEntryRequest, opts ...grpc.CallOption) (*filer_pb.CreateEntryResponse, error)
	deleteEntryFunc                     func(ctx context.Context, in *filer_pb.DeleteEntryRequest, opts ...grpc.CallOption) (*filer_pb.DeleteEntryResponse, error)
	appendToEntryFunc                   func(ctx context.Context, in *filer_pb.AppendToEntryRequest, opts ...grpc.CallOption) (*filer_pb.AppendToEntryResponse, error)
	lookupDirectoryEntryFunc            func(ctx context.Context, in *filer_pb.LookupDirectoryEntryRequest, opts ...grpc.CallOption) (*filer_pb.LookupDirectoryEntryResponse, error)
	updateEntryFunc                     func(ctx context.Context, in *filer_pb.UpdateEntryRequest, opts ...grpc.CallOption) (*filer_pb.UpdateEntryResponse, error)
	assignVolumeFunc                    func(ctx context.Context, in *filer_pb.AssignVolumeRequest, opts ...grpc.CallOption) (*filer_pb.AssignVolumeResponse, error)
	atomicRenameEntryFunc               func(ctx context.Context, in *filer_pb.AtomicRenameEntryRequest, opts ...grpc.CallOption) (*filer_pb.AtomicRenameEntryResponse, error)
	cacheRemoteObjectToLocalClusterFunc func(ctx context.Context, in *filer_pb.CacheRemoteObjectToLocalClusterRequest, opts ...grpc.CallOption) (*filer_pb.CacheRemoteObjectToLocalClusterResponse, error)
	deleteCollectionFunc                func(ctx context.Context, in *filer_pb.DeleteCollectionRequest, opts ...grpc.CallOption) (*filer_pb.DeleteCollectionResponse, error)
	collectionListFunc                  func(ctx context.Context, in *filer_pb.CollectionListRequest, opts ...grpc.CallOption) (*filer_pb.CollectionListResponse, error)
	distributedLockFunc                 func(ctx context.Context, in *filer_pb.LockRequest, opts ...grpc.CallOption) (*filer_pb.LockResponse, error)
	distributedUnlockFunc               func(ctx context.Context, in *filer_pb.UnlockRequest, opts ...grpc.CallOption) (*filer_pb.UnlockResponse, error)
	findLockOwnerFunc                   func(ctx context.Context, in *filer_pb.FindLockOwnerRequest, opts ...grpc.CallOption) (*filer_pb.FindLockOwnerResponse, error)
	listEntriesFunc                     func(ctx context.Context, in *filer_pb.ListEntriesRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_ListEntriesClient, error)
	streamRenameEntryFunc               func(ctx context.Context, in *filer_pb.StreamRenameEntryRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_StreamRenameEntryClient, error)
	lookupVolumeFunc                    func(ctx context.Context, in *filer_pb.LookupVolumeRequest, opts ...grpc.CallOption) (*filer_pb.LookupVolumeResponse, error)
	statisticsFunc                      func(ctx context.Context, in *filer_pb.StatisticsRequest, opts ...grpc.CallOption) (*filer_pb.StatisticsResponse, error)
	pingFunc                            func(ctx context.Context, in *filer_pb.PingRequest, opts ...grpc.CallOption) (*filer_pb.PingResponse, error)
	getFilerConfigurationFunc           func(ctx context.Context, in *filer_pb.GetFilerConfigurationRequest, opts ...grpc.CallOption) (*filer_pb.GetFilerConfigurationResponse, error)
	traverseBfsMetadataFunc             func(ctx context.Context, in *filer_pb.TraverseBfsMetadataRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_TraverseBfsMetadataClient, error)
	subscribeMetadataFunc               func(ctx context.Context, in *filer_pb.SubscribeMetadataRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_SubscribeMetadataClient, error)
	subscribeLocalMetadataFunc          func(ctx context.Context, in *filer_pb.SubscribeMetadataRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_SubscribeLocalMetadataClient, error)
	kvGetFunc                           func(ctx context.Context, in *filer_pb.KvGetRequest, opts ...grpc.CallOption) (*filer_pb.KvGetResponse, error)
	kvPutFunc                           func(ctx context.Context, in *filer_pb.KvPutRequest, opts ...grpc.CallOption) (*filer_pb.KvPutResponse, error)
	transferLocksFunc                   func(ctx context.Context, in *filer_pb.TransferLocksRequest, opts ...grpc.CallOption) (*filer_pb.TransferLocksResponse, error)
}

func (m *mockSeaweedFilerClient) LookupDirectoryEntry(ctx context.Context, in *filer_pb.LookupDirectoryEntryRequest, opts ...grpc.CallOption) (*filer_pb.LookupDirectoryEntryResponse, error) {
	if m.lookupDirectoryEntryFunc != nil {
		return m.lookupDirectoryEntryFunc(ctx, in, opts...)
	}
	return &filer_pb.LookupDirectoryEntryResponse{}, nil
}

func (m *mockSeaweedFilerClient) ListEntries(ctx context.Context, in *filer_pb.ListEntriesRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_ListEntriesClient, error) {
	if m.listEntriesFunc != nil {
		return m.listEntriesFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockSeaweedFilerClient) CreateEntry(ctx context.Context, in *filer_pb.CreateEntryRequest, opts ...grpc.CallOption) (*filer_pb.CreateEntryResponse, error) {
	if m.createEntryFunc != nil {
		return m.createEntryFunc(ctx, in, opts...)
	}
	return &filer_pb.CreateEntryResponse{}, nil
}

func (m *mockSeaweedFilerClient) UpdateEntry(ctx context.Context, in *filer_pb.UpdateEntryRequest, opts ...grpc.CallOption) (*filer_pb.UpdateEntryResponse, error) {
	if m.updateEntryFunc != nil {
		return m.updateEntryFunc(ctx, in, opts...)
	}
	return &filer_pb.UpdateEntryResponse{}, nil
}

func (m *mockSeaweedFilerClient) AppendToEntry(ctx context.Context, in *filer_pb.AppendToEntryRequest, opts ...grpc.CallOption) (*filer_pb.AppendToEntryResponse, error) {
	if m.appendToEntryFunc != nil {
		return m.appendToEntryFunc(ctx, in, opts...)
	}
	return &filer_pb.AppendToEntryResponse{}, nil
}

func (m *mockSeaweedFilerClient) DeleteEntry(ctx context.Context, in *filer_pb.DeleteEntryRequest, opts ...grpc.CallOption) (*filer_pb.DeleteEntryResponse, error) {
	if m.deleteEntryFunc != nil {
		return m.deleteEntryFunc(ctx, in, opts...)
	}
	return &filer_pb.DeleteEntryResponse{}, nil
}

func (m *mockSeaweedFilerClient) AtomicRenameEntry(ctx context.Context, in *filer_pb.AtomicRenameEntryRequest, opts ...grpc.CallOption) (*filer_pb.AtomicRenameEntryResponse, error) {
	if m.atomicRenameEntryFunc != nil {
		return m.atomicRenameEntryFunc(ctx, in, opts...)
	}
	return &filer_pb.AtomicRenameEntryResponse{}, nil
}

func (m *mockSeaweedFilerClient) StreamRenameEntry(ctx context.Context, in *filer_pb.StreamRenameEntryRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_StreamRenameEntryClient, error) {
	if m.streamRenameEntryFunc != nil {
		return m.streamRenameEntryFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockSeaweedFilerClient) AssignVolume(ctx context.Context, in *filer_pb.AssignVolumeRequest, opts ...grpc.CallOption) (*filer_pb.AssignVolumeResponse, error) {
	if m.assignVolumeFunc != nil {
		return m.assignVolumeFunc(ctx, in, opts...)
	}
	return &filer_pb.AssignVolumeResponse{}, nil
}

func (m *mockSeaweedFilerClient) LookupVolume(ctx context.Context, in *filer_pb.LookupVolumeRequest, opts ...grpc.CallOption) (*filer_pb.LookupVolumeResponse, error) {
	if m.lookupVolumeFunc != nil {
		return m.lookupVolumeFunc(ctx, in, opts...)
	}
	return &filer_pb.LookupVolumeResponse{}, nil
}

func (m *mockSeaweedFilerClient) CollectionList(ctx context.Context, in *filer_pb.CollectionListRequest, opts ...grpc.CallOption) (*filer_pb.CollectionListResponse, error) {
	if m.collectionListFunc != nil {
		return m.collectionListFunc(ctx, in, opts...)
	}
	return &filer_pb.CollectionListResponse{}, nil
}

func (m *mockSeaweedFilerClient) DeleteCollection(ctx context.Context, in *filer_pb.DeleteCollectionRequest, opts ...grpc.CallOption) (*filer_pb.DeleteCollectionResponse, error) {
	if m.deleteCollectionFunc != nil {
		return m.deleteCollectionFunc(ctx, in, opts...)
	}
	return &filer_pb.DeleteCollectionResponse{}, nil
}

func (m *mockSeaweedFilerClient) Statistics(ctx context.Context, in *filer_pb.StatisticsRequest, opts ...grpc.CallOption) (*filer_pb.StatisticsResponse, error) {
	if m.statisticsFunc != nil {
		return m.statisticsFunc(ctx, in, opts...)
	}
	return &filer_pb.StatisticsResponse{}, nil
}

func (m *mockSeaweedFilerClient) Ping(ctx context.Context, in *filer_pb.PingRequest, opts ...grpc.CallOption) (*filer_pb.PingResponse, error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx, in, opts...)
	}
	return &filer_pb.PingResponse{}, nil
}

func (m *mockSeaweedFilerClient) GetFilerConfiguration(ctx context.Context, in *filer_pb.GetFilerConfigurationRequest, opts ...grpc.CallOption) (*filer_pb.GetFilerConfigurationResponse, error) {
	if m.getFilerConfigurationFunc != nil {
		return m.getFilerConfigurationFunc(ctx, in, opts...)
	}
	return &filer_pb.GetFilerConfigurationResponse{}, nil
}

func (m *mockSeaweedFilerClient) TraverseBfsMetadata(ctx context.Context, in *filer_pb.TraverseBfsMetadataRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_TraverseBfsMetadataClient, error) {
	if m.traverseBfsMetadataFunc != nil {
		return m.traverseBfsMetadataFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockSeaweedFilerClient) SubscribeMetadata(ctx context.Context, in *filer_pb.SubscribeMetadataRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_SubscribeMetadataClient, error) {
	if m.subscribeMetadataFunc != nil {
		return m.subscribeMetadataFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockSeaweedFilerClient) SubscribeLocalMetadata(ctx context.Context, in *filer_pb.SubscribeMetadataRequest, opts ...grpc.CallOption) (filer_pb.SeaweedFiler_SubscribeLocalMetadataClient, error) {
	if m.subscribeLocalMetadataFunc != nil {
		return m.subscribeLocalMetadataFunc(ctx, in, opts...)
	}
	return nil, nil
}

func (m *mockSeaweedFilerClient) KvGet(ctx context.Context, in *filer_pb.KvGetRequest, opts ...grpc.CallOption) (*filer_pb.KvGetResponse, error) {
	if m.kvGetFunc != nil {
		return m.kvGetFunc(ctx, in, opts...)
	}
	return &filer_pb.KvGetResponse{}, nil
}

func (m *mockSeaweedFilerClient) KvPut(ctx context.Context, in *filer_pb.KvPutRequest, opts ...grpc.CallOption) (*filer_pb.KvPutResponse, error) {
	if m.kvPutFunc != nil {
		return m.kvPutFunc(ctx, in, opts...)
	}
	return &filer_pb.KvPutResponse{}, nil
}

func (m *mockSeaweedFilerClient) CacheRemoteObjectToLocalCluster(ctx context.Context, in *filer_pb.CacheRemoteObjectToLocalClusterRequest, opts ...grpc.CallOption) (*filer_pb.CacheRemoteObjectToLocalClusterResponse, error) {
	if m.cacheRemoteObjectToLocalClusterFunc != nil {
		return m.cacheRemoteObjectToLocalClusterFunc(ctx, in, opts...)
	}
	return &filer_pb.CacheRemoteObjectToLocalClusterResponse{}, nil
}

func (m *mockSeaweedFilerClient) DistributedLock(ctx context.Context, in *filer_pb.LockRequest, opts ...grpc.CallOption) (*filer_pb.LockResponse, error) {
	if m.distributedLockFunc != nil {
		return m.distributedLockFunc(ctx, in, opts...)
	}
	return &filer_pb.LockResponse{}, nil
}

func (m *mockSeaweedFilerClient) DistributedUnlock(ctx context.Context, in *filer_pb.UnlockRequest, opts ...grpc.CallOption) (*filer_pb.UnlockResponse, error) {
	if m.distributedUnlockFunc != nil {
		return m.distributedUnlockFunc(ctx, in, opts...)
	}
	return &filer_pb.UnlockResponse{}, nil
}

func (m *mockSeaweedFilerClient) FindLockOwner(ctx context.Context, in *filer_pb.FindLockOwnerRequest, opts ...grpc.CallOption) (*filer_pb.FindLockOwnerResponse, error) {
	if m.findLockOwnerFunc != nil {
		return m.findLockOwnerFunc(ctx, in, opts...)
	}
	return &filer_pb.FindLockOwnerResponse{}, nil
}

func (m *mockSeaweedFilerClient) TransferLocks(ctx context.Context, in *filer_pb.TransferLocksRequest, opts ...grpc.CallOption) (*filer_pb.TransferLocksResponse, error) {
	if m.transferLocksFunc != nil {
		return m.transferLocksFunc(ctx, in, opts...)
	}
	return &filer_pb.TransferLocksResponse{}, nil
}
