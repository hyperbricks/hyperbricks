package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPurgeConfiguredResponseCacheResolvesRouteAndPreservesOtherRoutes(t *testing.T) {
	setupLiveModeServeContentTest(t)
	for _, route := range []string{"products", "index"} {
		plugin := &liveCacheMatrixPlugin{}
		rm.SetPlugin(route, plugin)
		setTestRouteConfig(route, liveCacheMatrixRoute(route, route))
	}
	for _, route := range []string{"/products?id=1", "/products?id=2", "/"} {
		w := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, route, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("warm %s status = %d, body %s", route, w.Code, w.Body.String())
		}
	}
	if _, err := purgeConfiguredResponseCache("missing"); err == nil {
		t.Fatal("unknown route purge succeeded")
	}
	result, err := purgeConfiguredResponseCache("/products/")
	if err != nil || result.MemoryEntries != 2 || result.Bytes <= 0 {
		t.Fatalf("products purge = %#v, %v", result, err)
	}
	result, err = purgeConfiguredResponseCache("/")
	if err != nil || result.MemoryEntries != 1 {
		t.Fatalf("index purge = %#v, %v", result, err)
	}
	result, err = purgeConfiguredResponseCache("")
	if err != nil || result.MemoryEntries != 0 || result.DiskEntries != 0 {
		t.Fatalf("all purge after route purges = %#v, %v", result, err)
	}
}
