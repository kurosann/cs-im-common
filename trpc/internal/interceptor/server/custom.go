package server

import "google.golang.org/grpc"

var customInterceptors []grpc.UnaryServerInterceptor
var customStreamInterceptors []grpc.StreamServerInterceptor

func CustomServerInterceptors() []grpc.UnaryServerInterceptor {
	return customInterceptors
}
func CustomStreamServerInterceptors() []grpc.StreamServerInterceptor {
	return customStreamInterceptors
}
func RegisterCustomInterceptor(interceptor grpc.UnaryServerInterceptor) {
	customInterceptors = append(customInterceptors, interceptor)
}
func RegisterCustomStreamInterceptor(interceptor grpc.StreamServerInterceptor) {
	customStreamInterceptors = append(customStreamInterceptors, interceptor)
}
