package comm

import (
	"time"

	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Set the maximum number of bytes a message is allowed to
// carry to (256 * 1024 * 1024 B) + 2048 B (buffer) > 256 MiB.
// Symmetric - send and receive option.
var maxMsgSize = 268437504

// ReceiverOptions returns a list of gRPC server
// options that the internal receiver uses for RPCs.
func ReceiverOptions(tlsConfig *tls.Config) []grpc.ServerOption {

	// Use pluto-internal TLS config for credentials.
	creds := credentials.NewTLS(tlsConfig)

	// Use GZIP for compression and decompression.
	comp := grpc.NewGZIPCompressor()
	decomp := grpc.NewGZIPDecompressor()

	kaParams := keepalive.ServerParameters{
		// Any internal connection will be closed after
		// 5 minutes of being in idle state.
		MaxConnectionIdle: 5 * time.Minute,
		// The receiver will ping the other node after
		// 1 minute of inactivity for keepalive.
		Time: 1 * time.Minute,
		// If no response to such keepalive ping is received
		// after 30 seconds, the connection is closed.
		Timeout: 30 * time.Second,
	}

	enfPolicy := keepalive.EnforcementPolicy{
		// Clients connecting to this receiver should wait
		// at least 1 minute before sending a keepalive.
		MinTime: 1 * time.Minute,
		// Expect keepalives even when no streams are active.
		PermitWithoutStream: true,
	}

	// TODO: Think about clever stats handler. Prometheus-exposed?
	// stats := grpc.StatsHandler(h)

	return []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.RPCCompressor(comp),
		grpc.RPCDecompressor(decomp),
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
		grpc.KeepaliveParams(kaParams),
		grpc.KeepaliveEnforcementPolicy(enfPolicy),
		// grpc.StatsHandler(stats),
	}
}

// SenderOptions defines gRPC options for connection
// attempts from a sender to a receiver.
func SenderOptions(tlsConfig *tls.Config) []grpc.DialOption {

	// Use GZIP for compression and decompression.
	comp := grpc.NewGZIPCompressor()
	decomp := grpc.NewGZIPDecompressor()

	// These call options will be used for every call
	// via this connection.
	callOpts := []grpc.CallOption{
		// Fail immediately if connection is closed.
		grpc.FailFast(true),
		// Set maximum receive and send sizes.
		grpc.MaxCallRecvMsgSize(maxMsgSize),
		grpc.MaxCallSendMsgSize(maxMsgSize),
	}

	kaParams := keepalive.ClientParameters{
		// The client will ping the other node after
		// 1 minute of inactivity for keepalive.
		Time: 1 * time.Minute,
		// If no response to such keepalive ping is received
		// after 30 seconds, the connection is closed.
		Timeout: 30 * time.Second,
		// Expect keepalives even when no streams are active.
		PermitWithoutStream: true,
	}

	// TODO: Think about clever stats handler. Prometheus-exposed?
	// stats := grpc.StatsHandler(h)

	// Use pluto-internal TLS config for credentials.
	creds := credentials.NewTLS(tlsConfig)

	return []grpc.DialOption{
		grpc.WithCompressor(comp),
		grpc.WithDecompressor(decomp),
		grpc.WithBackoffMaxDelay(8 * time.Second),
		grpc.WithBlock(),
		grpc.WithTimeout(26 * time.Second),
		grpc.WithDefaultCallOptions(callOpts...),
		grpc.WithKeepaliveParams(kaParams),
		// grpc.WithStatsHandler(stats),
		grpc.WithTransportCredentials(creds),
	}
}
