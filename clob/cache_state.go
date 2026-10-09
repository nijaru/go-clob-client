package clob

import (
	"sync"
	"time"
)

// clientCacheState has one owner shared by read-only, signer, and authenticated
// views. Copying a view never copies a mutex or forks scalar cache state.
type clientCacheState struct {
	tickSizeMu              sync.RWMutex
	tickSizeGeneration      uint64
	tickSizeCache           map[string]TickSize
	tickSizeTimestamps      map[string]time.Time
	negRiskMu               sync.RWMutex
	negRiskGeneration       uint64
	negRiskCache            map[string]bool
	negRiskTimestamps       map[string]time.Time
	orderMetadataMu         sync.Mutex
	orderMetadataGeneration uint64
	orderConditionCache     map[string]string
	orderConditionLoads     map[string]*orderConditionLoad
	orderMetadataCache      map[string]orderMetadataEntry
	orderMetadataLoads      map[string]*orderMetadataLoad
	builderFeeCache         map[string]builderFeeEntry
	builderFeeLoads         map[string]*builderFeeLoad

	// versionMu guards cachedVersion, the lazily resolved CLOB server
	// protocol version (0 means uncached). It mirrors the Rust SDK's
	// resolve_version cache.
	versionMu     sync.RWMutex
	cachedVersion uint32
}

func newClientCacheState() *clientCacheState {
	return &clientCacheState{
		tickSizeCache:       make(map[string]TickSize),
		tickSizeTimestamps:  make(map[string]time.Time),
		negRiskCache:        make(map[string]bool),
		negRiskTimestamps:   make(map[string]time.Time),
		orderConditionCache: make(map[string]string),
		orderConditionLoads: make(map[string]*orderConditionLoad),
		orderMetadataCache:  make(map[string]orderMetadataEntry),
		orderMetadataLoads:  make(map[string]*orderMetadataLoad),
		builderFeeCache:     make(map[string]builderFeeEntry),
		builderFeeLoads:     make(map[string]*builderFeeLoad),
	}
}
