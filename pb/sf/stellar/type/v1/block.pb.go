// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        (unknown)
// source: sf/stellar/type/v1/block.proto

package pbstellar

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Block struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Number       uint64                 `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
	Hash         string                 `protobuf:"bytes,2,opt,name=hash,proto3" json:"hash,omitempty"`
	ParentHash   string                 `protobuf:"bytes,3,opt,name=parent_hash,json=parentHash,proto3" json:"parent_hash,omitempty"`
	Transactions []*Transaction         `protobuf:"bytes,5,rep,name=transactions,proto3" json:"transactions,omitempty"`
	Events       []*Event               `protobuf:"bytes,6,rep,name=events,proto3" json:"events,omitempty"`
	Timestamp    *timestamppb.Timestamp `protobuf:"bytes,9,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
}

func (x *Block) Reset() {
	*x = Block{}
	mi := &file_sf_stellar_type_v1_block_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Block) ProtoMessage() {}

func (x *Block) ProtoReflect() protoreflect.Message {
	mi := &file_sf_stellar_type_v1_block_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Block.ProtoReflect.Descriptor instead.
func (*Block) Descriptor() ([]byte, []int) {
	return file_sf_stellar_type_v1_block_proto_rawDescGZIP(), []int{0}
}

func (x *Block) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *Block) GetHash() string {
	if x != nil {
		return x.Hash
	}
	return ""
}

func (x *Block) GetParentHash() string {
	if x != nil {
		return x.ParentHash
	}
	return ""
}

func (x *Block) GetTransactions() []*Transaction {
	if x != nil {
		return x.Transactions
	}
	return nil
}

func (x *Block) GetEvents() []*Event {
	if x != nil {
		return x.Events
	}
	return nil
}

func (x *Block) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

type Transaction struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash   []byte `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Ledger uint64 `protobuf:"varint,2,opt,name=ledger,proto3" json:"ledger,omitempty"`
}

func (x *Transaction) Reset() {
	*x = Transaction{}
	mi := &file_sf_stellar_type_v1_block_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Transaction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Transaction) ProtoMessage() {}

func (x *Transaction) ProtoReflect() protoreflect.Message {
	mi := &file_sf_stellar_type_v1_block_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Transaction.ProtoReflect.Descriptor instead.
func (*Transaction) Descriptor() ([]byte, []int) {
	return file_sf_stellar_type_v1_block_proto_rawDescGZIP(), []int{1}
}

func (x *Transaction) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *Transaction) GetLedger() uint64 {
	if x != nil {
		return x.Ledger
	}
	return 0
}

type Event struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type                     string   `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
	Ledger                   int32    `protobuf:"varint,2,opt,name=ledger,proto3" json:"ledger,omitempty"`
	LedgerClosedAt           string   `protobuf:"bytes,3,opt,name=ledger_closed_at,json=ledgerClosedAt,proto3" json:"ledger_closed_at,omitempty"`
	ContractId               string   `protobuf:"bytes,4,opt,name=contract_id,json=contractId,proto3" json:"contract_id,omitempty"`
	Id                       string   `protobuf:"bytes,5,opt,name=id,proto3" json:"id,omitempty"`
	PagingToken              string   `protobuf:"bytes,6,opt,name=paging_token,json=pagingToken,proto3" json:"paging_token,omitempty"`
	InSuccessfulContractCall bool     `protobuf:"varint,7,opt,name=in_successful_contract_call,json=inSuccessfulContractCall,proto3" json:"in_successful_contract_call,omitempty"`
	Topic                    []string `protobuf:"bytes,8,rep,name=topic,proto3" json:"topic,omitempty"`
	Value                    string   `protobuf:"bytes,9,opt,name=value,proto3" json:"value,omitempty"`
	TxHash                   string   `protobuf:"bytes,10,opt,name=tx_hash,json=txHash,proto3" json:"tx_hash,omitempty"`
}

func (x *Event) Reset() {
	*x = Event{}
	mi := &file_sf_stellar_type_v1_block_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Event) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Event) ProtoMessage() {}

func (x *Event) ProtoReflect() protoreflect.Message {
	mi := &file_sf_stellar_type_v1_block_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Event.ProtoReflect.Descriptor instead.
func (*Event) Descriptor() ([]byte, []int) {
	return file_sf_stellar_type_v1_block_proto_rawDescGZIP(), []int{2}
}

func (x *Event) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Event) GetLedger() int32 {
	if x != nil {
		return x.Ledger
	}
	return 0
}

func (x *Event) GetLedgerClosedAt() string {
	if x != nil {
		return x.LedgerClosedAt
	}
	return ""
}

func (x *Event) GetContractId() string {
	if x != nil {
		return x.ContractId
	}
	return ""
}

func (x *Event) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Event) GetPagingToken() string {
	if x != nil {
		return x.PagingToken
	}
	return ""
}

func (x *Event) GetInSuccessfulContractCall() bool {
	if x != nil {
		return x.InSuccessfulContractCall
	}
	return false
}

func (x *Event) GetTopic() []string {
	if x != nil {
		return x.Topic
	}
	return nil
}

func (x *Event) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (x *Event) GetTxHash() string {
	if x != nil {
		return x.TxHash
	}
	return ""
}

var File_sf_stellar_type_v1_block_proto protoreflect.FileDescriptor

var file_sf_stellar_type_v1_block_proto_rawDesc = []byte{
	0x0a, 0x1e, 0x73, 0x66, 0x2f, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2f, 0x74, 0x79, 0x70,
	0x65, 0x2f, 0x76, 0x31, 0x2f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x12, 0x73, 0x66, 0x2e, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2e, 0x74, 0x79, 0x70,
	0x65, 0x2e, 0x76, 0x31, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x86, 0x02, 0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12,
	0x16, 0x0a, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x1f, 0x0a, 0x0b, 0x70,
	0x61, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0a, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x48, 0x61, 0x73, 0x68, 0x12, 0x43, 0x0a, 0x0c,
	0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x05, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x73, 0x66, 0x2e, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2e,
	0x74, 0x79, 0x70, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x0c, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x12, 0x31, 0x0a, 0x06, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x06, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x19, 0x2e, 0x73, 0x66, 0x2e, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2e, 0x74,
	0x79, 0x70, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x52, 0x06, 0x65, 0x76,
	0x65, 0x6e, 0x74, 0x73, 0x12, 0x38, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x22, 0x39,
	0x0a, 0x0b, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a,
	0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x68, 0x61, 0x73,
	0x68, 0x12, 0x16, 0x0a, 0x06, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x06, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72, 0x22, 0xb5, 0x02, 0x0a, 0x05, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x6c, 0x65, 0x64, 0x67, 0x65,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x06, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72, 0x12,
	0x28, 0x0a, 0x10, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72, 0x5f, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x64,
	0x5f, 0x61, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0e, 0x6c, 0x65, 0x64, 0x67, 0x65,
	0x72, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x64, 0x41, 0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x63, 0x6f, 0x6e,
	0x74, 0x72, 0x61, 0x63, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a,
	0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x49, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x21, 0x0a, 0x0c, 0x70, 0x61,
	0x67, 0x69, 0x6e, 0x67, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0b, 0x70, 0x61, 0x67, 0x69, 0x6e, 0x67, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x3d, 0x0a,
	0x1b, 0x69, 0x6e, 0x5f, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x66, 0x75, 0x6c, 0x5f, 0x63,
	0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x5f, 0x63, 0x61, 0x6c, 0x6c, 0x18, 0x07, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x18, 0x69, 0x6e, 0x53, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x66, 0x75, 0x6c,
	0x43, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x43, 0x61, 0x6c, 0x6c, 0x12, 0x14, 0x0a, 0x05,
	0x74, 0x6f, 0x70, 0x69, 0x63, 0x18, 0x08, 0x20, 0x03, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x70,
	0x69, 0x63, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x17, 0x0a, 0x07, 0x74, 0x78, 0x5f, 0x68,
	0x61, 0x73, 0x68, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x74, 0x78, 0x48, 0x61, 0x73,
	0x68, 0x42, 0x4b, 0x5a, 0x49, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x69, 0x6e, 0x67, 0x66, 0x61, 0x73, 0x74, 0x2f, 0x66, 0x69,
	0x72, 0x65, 0x68, 0x6f, 0x73, 0x65, 0x2d, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2f, 0x70,
	0x62, 0x2f, 0x73, 0x66, 0x2f, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2f, 0x74, 0x79, 0x70,
	0x65, 0x2f, 0x76, 0x31, 0x3b, 0x70, 0x62, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sf_stellar_type_v1_block_proto_rawDescOnce sync.Once
	file_sf_stellar_type_v1_block_proto_rawDescData = file_sf_stellar_type_v1_block_proto_rawDesc
)

func file_sf_stellar_type_v1_block_proto_rawDescGZIP() []byte {
	file_sf_stellar_type_v1_block_proto_rawDescOnce.Do(func() {
		file_sf_stellar_type_v1_block_proto_rawDescData = protoimpl.X.CompressGZIP(file_sf_stellar_type_v1_block_proto_rawDescData)
	})
	return file_sf_stellar_type_v1_block_proto_rawDescData
}

var file_sf_stellar_type_v1_block_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_sf_stellar_type_v1_block_proto_goTypes = []any{
	(*Block)(nil),                 // 0: sf.stellar.type.v1.Block
	(*Transaction)(nil),           // 1: sf.stellar.type.v1.Transaction
	(*Event)(nil),                 // 2: sf.stellar.type.v1.Event
	(*timestamppb.Timestamp)(nil), // 3: google.protobuf.Timestamp
}
var file_sf_stellar_type_v1_block_proto_depIdxs = []int32{
	1, // 0: sf.stellar.type.v1.Block.transactions:type_name -> sf.stellar.type.v1.Transaction
	2, // 1: sf.stellar.type.v1.Block.events:type_name -> sf.stellar.type.v1.Event
	3, // 2: sf.stellar.type.v1.Block.timestamp:type_name -> google.protobuf.Timestamp
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_sf_stellar_type_v1_block_proto_init() }
func file_sf_stellar_type_v1_block_proto_init() {
	if File_sf_stellar_type_v1_block_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_sf_stellar_type_v1_block_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_sf_stellar_type_v1_block_proto_goTypes,
		DependencyIndexes: file_sf_stellar_type_v1_block_proto_depIdxs,
		MessageInfos:      file_sf_stellar_type_v1_block_proto_msgTypes,
	}.Build()
	File_sf_stellar_type_v1_block_proto = out.File
	file_sf_stellar_type_v1_block_proto_rawDesc = nil
	file_sf_stellar_type_v1_block_proto_goTypes = nil
	file_sf_stellar_type_v1_block_proto_depIdxs = nil
}