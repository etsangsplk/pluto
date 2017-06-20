// Code generated by protoc-gen-go. DO NOT EDIT.
// source: node.proto

/*
Package imap is a generated protocol buffer package.

It is generated from these files:
	node.proto

It has these top-level messages:
	Context
	Confirmation
	Command
	Reply
	Await
	MailFile
*/
package imap

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Context struct {
	ClientID   string `protobuf:"bytes,1,opt,name=clientID" json:"clientID,omitempty"`
	UserName   string `protobuf:"bytes,2,opt,name=userName" json:"userName,omitempty"`
	RespWorker string `protobuf:"bytes,3,opt,name=respWorker" json:"respWorker,omitempty"`
}

func (m *Context) Reset()                    { *m = Context{} }
func (m *Context) String() string            { return proto.CompactTextString(m) }
func (*Context) ProtoMessage()               {}
func (*Context) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Context) GetClientID() string {
	if m != nil {
		return m.ClientID
	}
	return ""
}

func (m *Context) GetUserName() string {
	if m != nil {
		return m.UserName
	}
	return ""
}

func (m *Context) GetRespWorker() string {
	if m != nil {
		return m.RespWorker
	}
	return ""
}

type Confirmation struct {
	Status uint32 `protobuf:"varint,1,opt,name=status" json:"status,omitempty"`
}

func (m *Confirmation) Reset()                    { *m = Confirmation{} }
func (m *Confirmation) String() string            { return proto.CompactTextString(m) }
func (*Confirmation) ProtoMessage()               {}
func (*Confirmation) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *Confirmation) GetStatus() uint32 {
	if m != nil {
		return m.Status
	}
	return 0
}

type Command struct {
	Text     string `protobuf:"bytes,1,opt,name=text" json:"text,omitempty"`
	ClientID string `protobuf:"bytes,2,opt,name=clientID" json:"clientID,omitempty"`
}

func (m *Command) Reset()                    { *m = Command{} }
func (m *Command) String() string            { return proto.CompactTextString(m) }
func (*Command) ProtoMessage()               {}
func (*Command) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *Command) GetText() string {
	if m != nil {
		return m.Text
	}
	return ""
}

func (m *Command) GetClientID() string {
	if m != nil {
		return m.ClientID
	}
	return ""
}

type Reply struct {
	Text   string `protobuf:"bytes,1,opt,name=text" json:"text,omitempty"`
	Status uint32 `protobuf:"varint,2,opt,name=status" json:"status,omitempty"`
}

func (m *Reply) Reset()                    { *m = Reply{} }
func (m *Reply) String() string            { return proto.CompactTextString(m) }
func (*Reply) ProtoMessage()               {}
func (*Reply) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *Reply) GetText() string {
	if m != nil {
		return m.Text
	}
	return ""
}

func (m *Reply) GetStatus() uint32 {
	if m != nil {
		return m.Status
	}
	return 0
}

type Await struct {
	Text     string `protobuf:"bytes,1,opt,name=text" json:"text,omitempty"`
	Status   uint32 `protobuf:"varint,2,opt,name=status" json:"status,omitempty"`
	NumBytes uint32 `protobuf:"varint,3,opt,name=numBytes" json:"numBytes,omitempty"`
}

func (m *Await) Reset()                    { *m = Await{} }
func (m *Await) String() string            { return proto.CompactTextString(m) }
func (*Await) ProtoMessage()               {}
func (*Await) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *Await) GetText() string {
	if m != nil {
		return m.Text
	}
	return ""
}

func (m *Await) GetStatus() uint32 {
	if m != nil {
		return m.Status
	}
	return 0
}

func (m *Await) GetNumBytes() uint32 {
	if m != nil {
		return m.NumBytes
	}
	return 0
}

type MailFile struct {
	Content  []byte `protobuf:"bytes,1,opt,name=content,proto3" json:"content,omitempty"`
	ClientID string `protobuf:"bytes,2,opt,name=clientID" json:"clientID,omitempty"`
}

func (m *MailFile) Reset()                    { *m = MailFile{} }
func (m *MailFile) String() string            { return proto.CompactTextString(m) }
func (*MailFile) ProtoMessage()               {}
func (*MailFile) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *MailFile) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

func (m *MailFile) GetClientID() string {
	if m != nil {
		return m.ClientID
	}
	return ""
}

func init() {
	proto.RegisterType((*Context)(nil), "imap.Context")
	proto.RegisterType((*Confirmation)(nil), "imap.Confirmation")
	proto.RegisterType((*Command)(nil), "imap.Command")
	proto.RegisterType((*Reply)(nil), "imap.Reply")
	proto.RegisterType((*Await)(nil), "imap.Await")
	proto.RegisterType((*MailFile)(nil), "imap.MailFile")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Node service

type NodeClient interface {
	Prepare(ctx context.Context, in *Context, opts ...grpc.CallOption) (*Confirmation, error)
	Close(ctx context.Context, in *Context, opts ...grpc.CallOption) (*Confirmation, error)
	Select(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error)
	Create(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error)
	Delete(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error)
	List(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error)
	AppendBegin(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Await, error)
	AppendEnd(ctx context.Context, in *MailFile, opts ...grpc.CallOption) (*Reply, error)
	Expunge(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error)
	Store(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error)
}

type nodeClient struct {
	cc *grpc.ClientConn
}

func NewNodeClient(cc *grpc.ClientConn) NodeClient {
	return &nodeClient{cc}
}

func (c *nodeClient) Prepare(ctx context.Context, in *Context, opts ...grpc.CallOption) (*Confirmation, error) {
	out := new(Confirmation)
	err := grpc.Invoke(ctx, "/imap.Node/Prepare", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) Close(ctx context.Context, in *Context, opts ...grpc.CallOption) (*Confirmation, error) {
	out := new(Confirmation)
	err := grpc.Invoke(ctx, "/imap.Node/Close", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) Select(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error) {
	out := new(Reply)
	err := grpc.Invoke(ctx, "/imap.Node/Select", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) Create(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error) {
	out := new(Reply)
	err := grpc.Invoke(ctx, "/imap.Node/Create", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) Delete(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error) {
	out := new(Reply)
	err := grpc.Invoke(ctx, "/imap.Node/Delete", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) List(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error) {
	out := new(Reply)
	err := grpc.Invoke(ctx, "/imap.Node/List", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) AppendBegin(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Await, error) {
	out := new(Await)
	err := grpc.Invoke(ctx, "/imap.Node/AppendBegin", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) AppendEnd(ctx context.Context, in *MailFile, opts ...grpc.CallOption) (*Reply, error) {
	out := new(Reply)
	err := grpc.Invoke(ctx, "/imap.Node/AppendEnd", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) Expunge(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error) {
	out := new(Reply)
	err := grpc.Invoke(ctx, "/imap.Node/Expunge", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeClient) Store(ctx context.Context, in *Command, opts ...grpc.CallOption) (*Reply, error) {
	out := new(Reply)
	err := grpc.Invoke(ctx, "/imap.Node/Store", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Node service

type NodeServer interface {
	Prepare(context.Context, *Context) (*Confirmation, error)
	Close(context.Context, *Context) (*Confirmation, error)
	Select(context.Context, *Command) (*Reply, error)
	Create(context.Context, *Command) (*Reply, error)
	Delete(context.Context, *Command) (*Reply, error)
	List(context.Context, *Command) (*Reply, error)
	AppendBegin(context.Context, *Command) (*Await, error)
	AppendEnd(context.Context, *MailFile) (*Reply, error)
	Expunge(context.Context, *Command) (*Reply, error)
	Store(context.Context, *Command) (*Reply, error)
}

func RegisterNodeServer(s *grpc.Server, srv NodeServer) {
	s.RegisterService(&_Node_serviceDesc, srv)
}

func _Node_Prepare_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Context)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Prepare(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/Prepare",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Prepare(ctx, req.(*Context))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_Close_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Context)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Close(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/Close",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Close(ctx, req.(*Context))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_Select_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Command)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Select(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/Select",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Select(ctx, req.(*Command))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Command)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Create(ctx, req.(*Command))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Command)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Delete(ctx, req.(*Command))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Command)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).List(ctx, req.(*Command))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_AppendBegin_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Command)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).AppendBegin(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/AppendBegin",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).AppendBegin(ctx, req.(*Command))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_AppendEnd_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MailFile)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).AppendEnd(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/AppendEnd",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).AppendEnd(ctx, req.(*MailFile))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_Expunge_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Command)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Expunge(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/Expunge",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Expunge(ctx, req.(*Command))
	}
	return interceptor(ctx, in, info, handler)
}

func _Node_Store_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Command)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Store(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/imap.Node/Store",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Store(ctx, req.(*Command))
	}
	return interceptor(ctx, in, info, handler)
}

var _Node_serviceDesc = grpc.ServiceDesc{
	ServiceName: "imap.Node",
	HandlerType: (*NodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Prepare",
			Handler:    _Node_Prepare_Handler,
		},
		{
			MethodName: "Close",
			Handler:    _Node_Close_Handler,
		},
		{
			MethodName: "Select",
			Handler:    _Node_Select_Handler,
		},
		{
			MethodName: "Create",
			Handler:    _Node_Create_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _Node_Delete_Handler,
		},
		{
			MethodName: "List",
			Handler:    _Node_List_Handler,
		},
		{
			MethodName: "AppendBegin",
			Handler:    _Node_AppendBegin_Handler,
		},
		{
			MethodName: "AppendEnd",
			Handler:    _Node_AppendEnd_Handler,
		},
		{
			MethodName: "Expunge",
			Handler:    _Node_Expunge_Handler,
		},
		{
			MethodName: "Store",
			Handler:    _Node_Store_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "node.proto",
}

func init() { proto.RegisterFile("node.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 367 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x93, 0x4d, 0x4f, 0xf2, 0x40,
	0x14, 0x85, 0xf9, 0x68, 0x29, 0x5c, 0xe0, 0x5d, 0xcc, 0xe2, 0x4d, 0xc3, 0xc2, 0x98, 0x46, 0xd1,
	0xa8, 0xe9, 0x42, 0x56, 0xee, 0x84, 0x82, 0x89, 0x89, 0xa2, 0x29, 0x0b, 0xd7, 0x23, 0xbd, 0x92,
	0x89, 0xed, 0xcc, 0x64, 0x3a, 0x8d, 0xf0, 0x9b, 0xfc, 0x93, 0xa6, 0x2d, 0x14, 0xf1, 0xab, 0xba,
	0x3c, 0xbd, 0xcf, 0xcc, 0x3d, 0x73, 0x4e, 0x0a, 0xc0, 0x45, 0x80, 0xae, 0x54, 0x42, 0x0b, 0x62,
	0xb0, 0x88, 0x4a, 0x87, 0x82, 0xe5, 0x09, 0xae, 0x71, 0xa9, 0x49, 0x0f, 0x9a, 0xf3, 0x90, 0x21,
	0xd7, 0xd7, 0x63, 0xbb, 0xba, 0x5f, 0x3d, 0x6e, 0xf9, 0x85, 0x4e, 0x67, 0x49, 0x8c, 0x6a, 0x4a,
	0x23, 0xb4, 0x6b, 0xf9, 0x6c, 0xa3, 0xc9, 0x1e, 0x80, 0xc2, 0x58, 0x3e, 0x08, 0xf5, 0x8c, 0xca,
	0xae, 0x67, 0xd3, 0x77, 0x5f, 0x9c, 0x3e, 0x74, 0x3c, 0xc1, 0x9f, 0x98, 0x8a, 0xa8, 0x66, 0x82,
	0x93, 0xff, 0xd0, 0x88, 0x35, 0xd5, 0x49, 0x9c, 0x6d, 0xe9, 0xfa, 0x6b, 0xe5, 0x5c, 0xa4, 0x56,
	0xa2, 0x88, 0xf2, 0x80, 0x10, 0x30, 0x52, 0x4b, 0x6b, 0x1b, 0xc6, 0x27, 0x7b, 0xb5, 0x5d, 0x7b,
	0xce, 0x00, 0x4c, 0x1f, 0x65, 0xb8, 0xfa, 0xf2, 0xe0, 0x76, 0x5f, 0x6d, 0x67, 0xdf, 0x1d, 0x98,
	0xc3, 0x17, 0xca, 0xf4, 0x5f, 0x0e, 0xa5, 0x2e, 0x78, 0x12, 0x8d, 0x56, 0x1a, 0xe3, 0xec, 0xa9,
	0x5d, 0xbf, 0xd0, 0xce, 0x25, 0x34, 0x6f, 0x29, 0x0b, 0xaf, 0x58, 0x88, 0xc4, 0x06, 0x6b, 0x9e,
	0xe6, 0xca, 0xf3, 0x6b, 0x3b, 0xfe, 0x46, 0xfe, 0xf4, 0x8e, 0xf3, 0xd7, 0x3a, 0x18, 0x53, 0x11,
	0x20, 0x71, 0xc1, 0xba, 0x57, 0x28, 0xa9, 0x42, 0xd2, 0x75, 0xd3, 0xa2, 0xdc, 0x75, 0x4b, 0x3d,
	0x52, 0xc8, 0x22, 0x51, 0xa7, 0x42, 0xce, 0xc0, 0xf4, 0x42, 0x11, 0xff, 0x92, 0xee, 0x43, 0x63,
	0x86, 0x21, 0xce, 0xf5, 0x16, 0xcf, 0x72, 0xef, 0xb5, 0x73, 0x99, 0x65, 0x99, 0x73, 0x9e, 0x42,
	0xaa, 0xb1, 0x9c, 0x1b, 0x63, 0x88, 0xa5, 0xdc, 0x01, 0x18, 0x37, 0x2c, 0x2e, 0xdb, 0x7a, 0x0a,
	0xed, 0xa1, 0x94, 0xc8, 0x83, 0x11, 0x2e, 0x18, 0xff, 0x06, 0xce, 0x9a, 0x73, 0x2a, 0xe4, 0x04,
	0x5a, 0x39, 0x3c, 0xe1, 0x01, 0xf9, 0x97, 0xcf, 0x36, 0x25, 0x7c, 0xbc, 0xf8, 0x08, 0xac, 0xc9,
	0x52, 0x26, 0x7c, 0x51, 0xe6, 0xf3, 0x10, 0xcc, 0x99, 0x16, 0xaa, 0x04, 0x7b, 0x6c, 0x64, 0x3f,
	0xd2, 0xe0, 0x2d, 0x00, 0x00, 0xff, 0xff, 0x29, 0x24, 0x91, 0x9d, 0x56, 0x03, 0x00, 0x00,
}