package global_router

import (
	"go-commerce/middlewares"
	"net/http"
)



func GlobalRouter( mux *http.ServeMux) http.Handler{
	handleAllReq:=  func(w http.ResponseWriter, r *http.Request){
       if r.Method== "OPTIONS"{
			w.WriteHeader(200)
		}else{
			mux.ServeHTTP(w,r)
		}
	}

	return middlewares.HandleCorsMiddleware(http.HandlerFunc(handleAllReq))

}
