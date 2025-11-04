package elastictransport

import "net/http"

// RoundTripFunc denotes the function signature of the elastictransport.Interface#Perform method
type RoundTripFunc func(*http.Request) (*http.Response, error)

// InterceptorFunc is a function that takes a RoundTripFunc, next, and returns another RoundTripFunc.
// This allows a chain of Interceptors to be created.
type InterceptorFunc func(next RoundTripFunc) RoundTripFunc

// mergeInterceptors creates a new InterceptorFunc by combining an array of InterceptorFunc
// This allows us to create and store a single interceptor instead of recreating the chain on each request.
func mergeInterceptors(interceptors []InterceptorFunc) InterceptorFunc {
	return func(next RoundTripFunc) RoundTripFunc {
		if len(interceptors) == 0 {
			return next
		}
		var fn = next
		for i := len(interceptors) - 1; i >= 0; i-- {
			if interceptors[i] == nil {
				continue
			}
			fn = interceptors[i](fn)
		}
		return fn
	}
}
