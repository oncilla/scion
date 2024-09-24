// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: proto/hidden_segment/v1/hidden_segment.proto

package hidden_segmentconnect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	hidden_segment "github.com/scionproto/scion/pkg/proto/hidden_segment"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion1_13_0

const (
	// HiddenSegmentRegistrationServiceName is the fully-qualified name of the
	// HiddenSegmentRegistrationService service.
	HiddenSegmentRegistrationServiceName = "proto.hidden_segment.v1.HiddenSegmentRegistrationService"
	// HiddenSegmentLookupServiceName is the fully-qualified name of the HiddenSegmentLookupService
	// service.
	HiddenSegmentLookupServiceName = "proto.hidden_segment.v1.HiddenSegmentLookupService"
	// AuthoritativeHiddenSegmentLookupServiceName is the fully-qualified name of the
	// AuthoritativeHiddenSegmentLookupService service.
	AuthoritativeHiddenSegmentLookupServiceName = "proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// HiddenSegmentRegistrationServiceHiddenSegmentRegistrationProcedure is the fully-qualified name of
	// the HiddenSegmentRegistrationService's HiddenSegmentRegistration RPC.
	HiddenSegmentRegistrationServiceHiddenSegmentRegistrationProcedure = "/proto.hidden_segment.v1.HiddenSegmentRegistrationService/HiddenSegmentRegistration"
	// HiddenSegmentLookupServiceHiddenSegmentsProcedure is the fully-qualified name of the
	// HiddenSegmentLookupService's HiddenSegments RPC.
	HiddenSegmentLookupServiceHiddenSegmentsProcedure = "/proto.hidden_segment.v1.HiddenSegmentLookupService/HiddenSegments"
	// AuthoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsProcedure is the
	// fully-qualified name of the AuthoritativeHiddenSegmentLookupService's AuthoritativeHiddenSegments
	// RPC.
	AuthoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsProcedure = "/proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService/AuthoritativeHiddenSegments"
)

// These variables are the protoreflect.Descriptor objects for the RPCs defined in this package.
var (
	hiddenSegmentRegistrationServiceServiceDescriptor                                  = hidden_segment.File_proto_hidden_segment_v1_hidden_segment_proto.Services().ByName("HiddenSegmentRegistrationService")
	hiddenSegmentRegistrationServiceHiddenSegmentRegistrationMethodDescriptor          = hiddenSegmentRegistrationServiceServiceDescriptor.Methods().ByName("HiddenSegmentRegistration")
	hiddenSegmentLookupServiceServiceDescriptor                                        = hidden_segment.File_proto_hidden_segment_v1_hidden_segment_proto.Services().ByName("HiddenSegmentLookupService")
	hiddenSegmentLookupServiceHiddenSegmentsMethodDescriptor                           = hiddenSegmentLookupServiceServiceDescriptor.Methods().ByName("HiddenSegments")
	authoritativeHiddenSegmentLookupServiceServiceDescriptor                           = hidden_segment.File_proto_hidden_segment_v1_hidden_segment_proto.Services().ByName("AuthoritativeHiddenSegmentLookupService")
	authoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsMethodDescriptor = authoritativeHiddenSegmentLookupServiceServiceDescriptor.Methods().ByName("AuthoritativeHiddenSegments")
)

// HiddenSegmentRegistrationServiceClient is a client for the
// proto.hidden_segment.v1.HiddenSegmentRegistrationService service.
type HiddenSegmentRegistrationServiceClient interface {
	HiddenSegmentRegistration(context.Context, *connect.Request[hidden_segment.HiddenSegmentRegistrationRequest]) (*connect.Response[hidden_segment.HiddenSegmentRegistrationResponse], error)
}

// NewHiddenSegmentRegistrationServiceClient constructs a client for the
// proto.hidden_segment.v1.HiddenSegmentRegistrationService service. By default, it uses the Connect
// protocol with the binary Protobuf Codec, asks for gzipped responses, and sends uncompressed
// requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewHiddenSegmentRegistrationServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) HiddenSegmentRegistrationServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &hiddenSegmentRegistrationServiceClient{
		hiddenSegmentRegistration: connect.NewClient[hidden_segment.HiddenSegmentRegistrationRequest, hidden_segment.HiddenSegmentRegistrationResponse](
			httpClient,
			baseURL+HiddenSegmentRegistrationServiceHiddenSegmentRegistrationProcedure,
			connect.WithSchema(hiddenSegmentRegistrationServiceHiddenSegmentRegistrationMethodDescriptor),
			connect.WithClientOptions(opts...),
		),
	}
}

// hiddenSegmentRegistrationServiceClient implements HiddenSegmentRegistrationServiceClient.
type hiddenSegmentRegistrationServiceClient struct {
	hiddenSegmentRegistration *connect.Client[hidden_segment.HiddenSegmentRegistrationRequest, hidden_segment.HiddenSegmentRegistrationResponse]
}

// HiddenSegmentRegistration calls
// proto.hidden_segment.v1.HiddenSegmentRegistrationService.HiddenSegmentRegistration.
func (c *hiddenSegmentRegistrationServiceClient) HiddenSegmentRegistration(ctx context.Context, req *connect.Request[hidden_segment.HiddenSegmentRegistrationRequest]) (*connect.Response[hidden_segment.HiddenSegmentRegistrationResponse], error) {
	return c.hiddenSegmentRegistration.CallUnary(ctx, req)
}

// HiddenSegmentRegistrationServiceHandler is an implementation of the
// proto.hidden_segment.v1.HiddenSegmentRegistrationService service.
type HiddenSegmentRegistrationServiceHandler interface {
	HiddenSegmentRegistration(context.Context, *connect.Request[hidden_segment.HiddenSegmentRegistrationRequest]) (*connect.Response[hidden_segment.HiddenSegmentRegistrationResponse], error)
}

// NewHiddenSegmentRegistrationServiceHandler builds an HTTP handler from the service
// implementation. It returns the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewHiddenSegmentRegistrationServiceHandler(svc HiddenSegmentRegistrationServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	hiddenSegmentRegistrationServiceHiddenSegmentRegistrationHandler := connect.NewUnaryHandler(
		HiddenSegmentRegistrationServiceHiddenSegmentRegistrationProcedure,
		svc.HiddenSegmentRegistration,
		connect.WithSchema(hiddenSegmentRegistrationServiceHiddenSegmentRegistrationMethodDescriptor),
		connect.WithHandlerOptions(opts...),
	)
	return "/proto.hidden_segment.v1.HiddenSegmentRegistrationService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case HiddenSegmentRegistrationServiceHiddenSegmentRegistrationProcedure:
			hiddenSegmentRegistrationServiceHiddenSegmentRegistrationHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedHiddenSegmentRegistrationServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedHiddenSegmentRegistrationServiceHandler struct{}

func (UnimplementedHiddenSegmentRegistrationServiceHandler) HiddenSegmentRegistration(context.Context, *connect.Request[hidden_segment.HiddenSegmentRegistrationRequest]) (*connect.Response[hidden_segment.HiddenSegmentRegistrationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("proto.hidden_segment.v1.HiddenSegmentRegistrationService.HiddenSegmentRegistration is not implemented"))
}

// HiddenSegmentLookupServiceClient is a client for the
// proto.hidden_segment.v1.HiddenSegmentLookupService service.
type HiddenSegmentLookupServiceClient interface {
	HiddenSegments(context.Context, *connect.Request[hidden_segment.HiddenSegmentsRequest]) (*connect.Response[hidden_segment.HiddenSegmentsResponse], error)
}

// NewHiddenSegmentLookupServiceClient constructs a client for the
// proto.hidden_segment.v1.HiddenSegmentLookupService service. By default, it uses the Connect
// protocol with the binary Protobuf Codec, asks for gzipped responses, and sends uncompressed
// requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewHiddenSegmentLookupServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) HiddenSegmentLookupServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &hiddenSegmentLookupServiceClient{
		hiddenSegments: connect.NewClient[hidden_segment.HiddenSegmentsRequest, hidden_segment.HiddenSegmentsResponse](
			httpClient,
			baseURL+HiddenSegmentLookupServiceHiddenSegmentsProcedure,
			connect.WithSchema(hiddenSegmentLookupServiceHiddenSegmentsMethodDescriptor),
			connect.WithClientOptions(opts...),
		),
	}
}

// hiddenSegmentLookupServiceClient implements HiddenSegmentLookupServiceClient.
type hiddenSegmentLookupServiceClient struct {
	hiddenSegments *connect.Client[hidden_segment.HiddenSegmentsRequest, hidden_segment.HiddenSegmentsResponse]
}

// HiddenSegments calls proto.hidden_segment.v1.HiddenSegmentLookupService.HiddenSegments.
func (c *hiddenSegmentLookupServiceClient) HiddenSegments(ctx context.Context, req *connect.Request[hidden_segment.HiddenSegmentsRequest]) (*connect.Response[hidden_segment.HiddenSegmentsResponse], error) {
	return c.hiddenSegments.CallUnary(ctx, req)
}

// HiddenSegmentLookupServiceHandler is an implementation of the
// proto.hidden_segment.v1.HiddenSegmentLookupService service.
type HiddenSegmentLookupServiceHandler interface {
	HiddenSegments(context.Context, *connect.Request[hidden_segment.HiddenSegmentsRequest]) (*connect.Response[hidden_segment.HiddenSegmentsResponse], error)
}

// NewHiddenSegmentLookupServiceHandler builds an HTTP handler from the service implementation. It
// returns the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewHiddenSegmentLookupServiceHandler(svc HiddenSegmentLookupServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	hiddenSegmentLookupServiceHiddenSegmentsHandler := connect.NewUnaryHandler(
		HiddenSegmentLookupServiceHiddenSegmentsProcedure,
		svc.HiddenSegments,
		connect.WithSchema(hiddenSegmentLookupServiceHiddenSegmentsMethodDescriptor),
		connect.WithHandlerOptions(opts...),
	)
	return "/proto.hidden_segment.v1.HiddenSegmentLookupService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case HiddenSegmentLookupServiceHiddenSegmentsProcedure:
			hiddenSegmentLookupServiceHiddenSegmentsHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedHiddenSegmentLookupServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedHiddenSegmentLookupServiceHandler struct{}

func (UnimplementedHiddenSegmentLookupServiceHandler) HiddenSegments(context.Context, *connect.Request[hidden_segment.HiddenSegmentsRequest]) (*connect.Response[hidden_segment.HiddenSegmentsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("proto.hidden_segment.v1.HiddenSegmentLookupService.HiddenSegments is not implemented"))
}

// AuthoritativeHiddenSegmentLookupServiceClient is a client for the
// proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService service.
type AuthoritativeHiddenSegmentLookupServiceClient interface {
	AuthoritativeHiddenSegments(context.Context, *connect.Request[hidden_segment.AuthoritativeHiddenSegmentsRequest]) (*connect.Response[hidden_segment.AuthoritativeHiddenSegmentsResponse], error)
}

// NewAuthoritativeHiddenSegmentLookupServiceClient constructs a client for the
// proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService service. By default, it uses the
// Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and sends
// uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewAuthoritativeHiddenSegmentLookupServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) AuthoritativeHiddenSegmentLookupServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &authoritativeHiddenSegmentLookupServiceClient{
		authoritativeHiddenSegments: connect.NewClient[hidden_segment.AuthoritativeHiddenSegmentsRequest, hidden_segment.AuthoritativeHiddenSegmentsResponse](
			httpClient,
			baseURL+AuthoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsProcedure,
			connect.WithSchema(authoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsMethodDescriptor),
			connect.WithClientOptions(opts...),
		),
	}
}

// authoritativeHiddenSegmentLookupServiceClient implements
// AuthoritativeHiddenSegmentLookupServiceClient.
type authoritativeHiddenSegmentLookupServiceClient struct {
	authoritativeHiddenSegments *connect.Client[hidden_segment.AuthoritativeHiddenSegmentsRequest, hidden_segment.AuthoritativeHiddenSegmentsResponse]
}

// AuthoritativeHiddenSegments calls
// proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService.AuthoritativeHiddenSegments.
func (c *authoritativeHiddenSegmentLookupServiceClient) AuthoritativeHiddenSegments(ctx context.Context, req *connect.Request[hidden_segment.AuthoritativeHiddenSegmentsRequest]) (*connect.Response[hidden_segment.AuthoritativeHiddenSegmentsResponse], error) {
	return c.authoritativeHiddenSegments.CallUnary(ctx, req)
}

// AuthoritativeHiddenSegmentLookupServiceHandler is an implementation of the
// proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService service.
type AuthoritativeHiddenSegmentLookupServiceHandler interface {
	AuthoritativeHiddenSegments(context.Context, *connect.Request[hidden_segment.AuthoritativeHiddenSegmentsRequest]) (*connect.Response[hidden_segment.AuthoritativeHiddenSegmentsResponse], error)
}

// NewAuthoritativeHiddenSegmentLookupServiceHandler builds an HTTP handler from the service
// implementation. It returns the path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewAuthoritativeHiddenSegmentLookupServiceHandler(svc AuthoritativeHiddenSegmentLookupServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	authoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsHandler := connect.NewUnaryHandler(
		AuthoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsProcedure,
		svc.AuthoritativeHiddenSegments,
		connect.WithSchema(authoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsMethodDescriptor),
		connect.WithHandlerOptions(opts...),
	)
	return "/proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case AuthoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsProcedure:
			authoritativeHiddenSegmentLookupServiceAuthoritativeHiddenSegmentsHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedAuthoritativeHiddenSegmentLookupServiceHandler returns CodeUnimplemented from all
// methods.
type UnimplementedAuthoritativeHiddenSegmentLookupServiceHandler struct{}

func (UnimplementedAuthoritativeHiddenSegmentLookupServiceHandler) AuthoritativeHiddenSegments(context.Context, *connect.Request[hidden_segment.AuthoritativeHiddenSegmentsRequest]) (*connect.Response[hidden_segment.AuthoritativeHiddenSegmentsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("proto.hidden_segment.v1.AuthoritativeHiddenSegmentLookupService.AuthoritativeHiddenSegments is not implemented"))
}
