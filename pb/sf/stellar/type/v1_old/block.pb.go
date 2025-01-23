// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        (unknown)
// source: sf/stellar/type/v1_old/block.proto

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
	Hash         []byte                 `protobuf:"bytes,2,opt,name=hash,proto3" json:"hash,omitempty"`
	Header       *Header                `protobuf:"bytes,3,opt,name=header,proto3" json:"header,omitempty"`
	Transactions []*Transaction         `protobuf:"bytes,6,rep,name=transactions,proto3" json:"transactions,omitempty"`
	CreatedAt    *timestamppb.Timestamp `protobuf:"bytes,9,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
}

func (x *Block) Reset() {
	*x = Block{}
	mi := &file_sf_stellar_type_v1_old_block_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Block) ProtoMessage() {}

func (x *Block) ProtoReflect() protoreflect.Message {
	mi := &file_sf_stellar_type_v1_old_block_proto_msgTypes[0]
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
	return file_sf_stellar_type_v1_old_block_proto_rawDescGZIP(), []int{0}
}

func (x *Block) GetNumber() uint64 {
	if x != nil {
		return x.Number
	}
	return 0
}

func (x *Block) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *Block) GetHeader() *Header {
	if x != nil {
		return x.Header
	}
	return nil
}

func (x *Block) GetTransactions() []*Transaction {
	if x != nil {
		return x.Transactions
	}
	return nil
}

func (x *Block) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.CreatedAt
	}
	return nil
}

type Header struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	LedgerVersion      uint32 `protobuf:"varint,1,opt,name=ledger_version,json=ledgerVersion,proto3" json:"ledger_version,omitempty"`
	PreviousLedgerHash []byte `protobuf:"bytes,2,opt,name=previous_ledger_hash,json=previousLedgerHash,proto3" json:"previous_ledger_hash,omitempty"`
	TotalCoins         int64  `protobuf:"varint,3,opt,name=total_coins,json=totalCoins,proto3" json:"total_coins,omitempty"` // The amount of stroops in existence at the end of the ledger
	BaseFee            uint32 `protobuf:"varint,4,opt,name=base_fee,json=baseFee,proto3" json:"base_fee,omitempty"`
	BaseReserve        uint32 `protobuf:"varint,5,opt,name=base_reserve,json=baseReserve,proto3" json:"base_reserve,omitempty"`
}

func (x *Header) Reset() {
	*x = Header{}
	mi := &file_sf_stellar_type_v1_old_block_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Header) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Header) ProtoMessage() {}

func (x *Header) ProtoReflect() protoreflect.Message {
	mi := &file_sf_stellar_type_v1_old_block_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Header.ProtoReflect.Descriptor instead.
func (*Header) Descriptor() ([]byte, []int) {
	return file_sf_stellar_type_v1_old_block_proto_rawDescGZIP(), []int{1}
}

func (x *Header) GetLedgerVersion() uint32 {
	if x != nil {
		return x.LedgerVersion
	}
	return 0
}

func (x *Header) GetPreviousLedgerHash() []byte {
	if x != nil {
		return x.PreviousLedgerHash
	}
	return nil
}

func (x *Header) GetTotalCoins() int64 {
	if x != nil {
		return x.TotalCoins
	}
	return 0
}

func (x *Header) GetBaseFee() uint32 {
	if x != nil {
		return x.BaseFee
	}
	return 0
}

func (x *Header) GetBaseReserve() uint32 {
	if x != nil {
		return x.BaseReserve
	}
	return 0
}

type Transaction struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash             []byte                 `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Status           string                 `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"`
	CreatedAt        *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
	ApplicationOrder uint64                 `protobuf:"varint,5,opt,name=application_order,json=applicationOrder,proto3" json:"application_order,omitempty"`
	EnvelopeXdr      []byte                 `protobuf:"bytes,6,opt,name=envelope_xdr,json=envelopeXdr,proto3" json:"envelope_xdr,omitempty"`
	ResultMetaXdr    []byte                 `protobuf:"bytes,7,opt,name=result_meta_xdr,json=resultMetaXdr,proto3" json:"result_meta_xdr,omitempty"`
	ResultXdr        []byte                 `protobuf:"bytes,8,opt,name=result_xdr,json=resultXdr,proto3" json:"result_xdr,omitempty"`
}

func (x *Transaction) Reset() {
	*x = Transaction{}
	mi := &file_sf_stellar_type_v1_old_block_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Transaction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Transaction) ProtoMessage() {}

func (x *Transaction) ProtoReflect() protoreflect.Message {
	mi := &file_sf_stellar_type_v1_old_block_proto_msgTypes[2]
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
	return file_sf_stellar_type_v1_old_block_proto_rawDescGZIP(), []int{2}
}

func (x *Transaction) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *Transaction) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *Transaction) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.CreatedAt
	}
	return nil
}

func (x *Transaction) GetApplicationOrder() uint64 {
	if x != nil {
		return x.ApplicationOrder
	}
	return 0
}

func (x *Transaction) GetEnvelopeXdr() []byte {
	if x != nil {
		return x.EnvelopeXdr
	}
	return nil
}

func (x *Transaction) GetResultMetaXdr() []byte {
	if x != nil {
		return x.ResultMetaXdr
	}
	return nil
}

func (x *Transaction) GetResultXdr() []byte {
	if x != nil {
		return x.ResultXdr
	}
	return nil
}

var File_sf_stellar_type_v1_old_block_proto protoreflect.FileDescriptor

var file_sf_stellar_type_v1_old_block_proto_rawDesc = []byte{
	0x0a, 0x22, 0x73, 0x66, 0x2f, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2f, 0x74, 0x79, 0x70,
	0x65, 0x2f, 0x76, 0x31, 0x5f, 0x6f, 0x6c, 0x64, 0x2f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x16, 0x73, 0x66, 0x2e, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72,
	0x2e, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x76, 0x31, 0x5f, 0x6f, 0x6c, 0x64, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xef, 0x01,
	0x0a, 0x05, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x16, 0x0a, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65,
	0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x6e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x12,
	0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x68,
	0x61, 0x73, 0x68, 0x12, 0x36, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x73, 0x66, 0x2e, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72,
	0x2e, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x76, 0x31, 0x5f, 0x6f, 0x6c, 0x64, 0x2e, 0x48, 0x65, 0x61,
	0x64, 0x65, 0x72, 0x52, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x47, 0x0a, 0x0c, 0x74,
	0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x06, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x23, 0x2e, 0x73, 0x66, 0x2e, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2e, 0x74,
	0x79, 0x70, 0x65, 0x2e, 0x76, 0x31, 0x5f, 0x6f, 0x6c, 0x64, 0x2e, 0x54, 0x72, 0x61, 0x6e, 0x73,
	0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0c, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x12, 0x39, 0x0a, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x5f,
	0x61, 0x74, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x22,
	0xc0, 0x01, 0x0a, 0x06, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x25, 0x0a, 0x0e, 0x6c, 0x65,
	0x64, 0x67, 0x65, 0x72, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x0d, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f,
	0x6e, 0x12, 0x30, 0x0a, 0x14, 0x70, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x5f, 0x6c, 0x65,
	0x64, 0x67, 0x65, 0x72, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x12, 0x70, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x4c, 0x65, 0x64, 0x67, 0x65, 0x72, 0x48,
	0x61, 0x73, 0x68, 0x12, 0x1f, 0x0a, 0x0b, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x5f, 0x63, 0x6f, 0x69,
	0x6e, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0a, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x43,
	0x6f, 0x69, 0x6e, 0x73, 0x12, 0x19, 0x0a, 0x08, 0x62, 0x61, 0x73, 0x65, 0x5f, 0x66, 0x65, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x07, 0x62, 0x61, 0x73, 0x65, 0x46, 0x65, 0x65, 0x12,
	0x21, 0x0a, 0x0c, 0x62, 0x61, 0x73, 0x65, 0x5f, 0x72, 0x65, 0x73, 0x65, 0x72, 0x76, 0x65, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0b, 0x62, 0x61, 0x73, 0x65, 0x52, 0x65, 0x73, 0x65, 0x72,
	0x76, 0x65, 0x22, 0x8b, 0x02, 0x0a, 0x0b, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x39,
	0x0a, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09,
	0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x12, 0x2b, 0x0a, 0x11, 0x61, 0x70, 0x70,
	0x6c, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x10, 0x61, 0x70, 0x70, 0x6c, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x12, 0x21, 0x0a, 0x0c, 0x65, 0x6e, 0x76, 0x65, 0x6c, 0x6f,
	0x70, 0x65, 0x5f, 0x78, 0x64, 0x72, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x65, 0x6e,
	0x76, 0x65, 0x6c, 0x6f, 0x70, 0x65, 0x58, 0x64, 0x72, 0x12, 0x26, 0x0a, 0x0f, 0x72, 0x65, 0x73,
	0x75, 0x6c, 0x74, 0x5f, 0x6d, 0x65, 0x74, 0x61, 0x5f, 0x78, 0x64, 0x72, 0x18, 0x07, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x0d, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x4d, 0x65, 0x74, 0x61, 0x58, 0x64,
	0x72, 0x12, 0x1d, 0x0a, 0x0a, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x5f, 0x78, 0x64, 0x72, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x58, 0x64, 0x72,
	0x42, 0x4f, 0x5a, 0x4d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x69, 0x6e, 0x67, 0x66, 0x61, 0x73, 0x74, 0x2f, 0x66, 0x69, 0x72,
	0x65, 0x68, 0x6f, 0x73, 0x65, 0x2d, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2f, 0x70, 0x62,
	0x2f, 0x73, 0x66, 0x2f, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61, 0x72, 0x2f, 0x74, 0x79, 0x70, 0x65,
	0x2f, 0x76, 0x31, 0x5f, 0x6f, 0x6c, 0x64, 0x3b, 0x70, 0x62, 0x73, 0x74, 0x65, 0x6c, 0x6c, 0x61,
	0x72, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sf_stellar_type_v1_old_block_proto_rawDescOnce sync.Once
	file_sf_stellar_type_v1_old_block_proto_rawDescData = file_sf_stellar_type_v1_old_block_proto_rawDesc
)

func file_sf_stellar_type_v1_old_block_proto_rawDescGZIP() []byte {
	file_sf_stellar_type_v1_old_block_proto_rawDescOnce.Do(func() {
		file_sf_stellar_type_v1_old_block_proto_rawDescData = protoimpl.X.CompressGZIP(file_sf_stellar_type_v1_old_block_proto_rawDescData)
	})
	return file_sf_stellar_type_v1_old_block_proto_rawDescData
}

var file_sf_stellar_type_v1_old_block_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_sf_stellar_type_v1_old_block_proto_goTypes = []any{
	(*Block)(nil),                 // 0: sf.stellar.type.v1_old.Block
	(*Header)(nil),                // 1: sf.stellar.type.v1_old.Header
	(*Transaction)(nil),           // 2: sf.stellar.type.v1_old.Transaction
	(*timestamppb.Timestamp)(nil), // 3: google.protobuf.Timestamp
}
var file_sf_stellar_type_v1_old_block_proto_depIdxs = []int32{
	1, // 0: sf.stellar.type.v1_old.Block.header:type_name -> sf.stellar.type.v1_old.Header
	2, // 1: sf.stellar.type.v1_old.Block.transactions:type_name -> sf.stellar.type.v1_old.Transaction
	3, // 2: sf.stellar.type.v1_old.Block.created_at:type_name -> google.protobuf.Timestamp
	3, // 3: sf.stellar.type.v1_old.Transaction.created_at:type_name -> google.protobuf.Timestamp
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_sf_stellar_type_v1_old_block_proto_init() }
func file_sf_stellar_type_v1_old_block_proto_init() {
	if File_sf_stellar_type_v1_old_block_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_sf_stellar_type_v1_old_block_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_sf_stellar_type_v1_old_block_proto_goTypes,
		DependencyIndexes: file_sf_stellar_type_v1_old_block_proto_depIdxs,
		MessageInfos:      file_sf_stellar_type_v1_old_block_proto_msgTypes,
	}.Build()
	File_sf_stellar_type_v1_old_block_proto = out.File
	file_sf_stellar_type_v1_old_block_proto_rawDesc = nil
	file_sf_stellar_type_v1_old_block_proto_goTypes = nil
	file_sf_stellar_type_v1_old_block_proto_depIdxs = nil
}
