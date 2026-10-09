package gamma

import (
	"time"
)

// Pagination describes offset-based pagination metadata in list/search responses.
type Pagination struct {
	HasMore      bool `json:"hasMore,omitzero"`
	TotalResults int  `json:"totalResults,omitzero"`
}

// PublicProfile represents a user's public metadata.
type PublicProfile struct {
	Address               string              `json:"address"`
	Name                  string              `json:"name,omitzero"`
	Bio                   string              `json:"bio,omitzero"`
	ProfileImage          string              `json:"profileImage,omitzero"`
	ProfileImageOptimized string              `json:"profileImageOptimized,omitzero"`
	ProxyWallet           string              `json:"proxyWallet,omitzero"`
	Pseudonym             string              `json:"pseudonym,omitzero"`
	Verified              bool                `json:"verified"`
	CreatedAt             time.Time           `json:"createdAt"`
	Users                 []PublicProfileUser `json:"users,omitzero"`
	XUsername             string              `json:"xUsername,omitzero"`
	VerifiedBadge         bool                `json:"verifiedBadge"`
	DisplayUsernamePublic bool                `json:"displayUsernamePublic,omitzero"`
}

// PublicProfileUser represents a user record in a public profile.
type PublicProfileUser struct {
	ID           string `json:"id"`
	Creator      bool   `json:"creator"`
	Mod          bool   `json:"mod"`
	CommunityMod bool   `json:"communityMod,omitzero"`
}

// SearchTag is a tag entry returned by Gamma public search.
type SearchTag struct {
	ID         string `json:"id,omitzero"`
	Label      string `json:"label,omitzero"`
	Slug       string `json:"slug,omitzero"`
	EventCount int    `json:"eventCount,omitzero"`
}

// Profile is a user/profile entry returned by Gamma public search.
type Profile struct {
	ID                     string             `json:"id"`
	Name                   string             `json:"name,omitzero"`
	User                   int                `json:"user,omitzero"`
	Referral               string             `json:"referral,omitzero"`
	CreatedBy              int                `json:"createdBy,omitzero"`
	UpdatedBy              int                `json:"updatedBy,omitzero"`
	CreatedAt              time.Time          `json:"createdAt,omitzero"`
	UpdatedAt              time.Time          `json:"updatedAt,omitzero"`
	UTMSource              string             `json:"utmSource,omitzero"`
	UTMMedium              string             `json:"utmMedium,omitzero"`
	UTMCampaign            string             `json:"utmCampaign,omitzero"`
	UTMContent             string             `json:"utmContent,omitzero"`
	UTMTerm                string             `json:"utmTerm,omitzero"`
	WalletActivated        bool               `json:"walletActivated,omitzero"`
	Pseudonym              string             `json:"pseudonym,omitzero"`
	DisplayUsernamePublic  bool               `json:"displayUsernamePublic,omitzero"`
	ProfileImage           string             `json:"profileImage,omitzero"`
	Bio                    string             `json:"bio,omitzero"`
	ProxyWallet            string             `json:"proxyWallet,omitzero"`
	ProfileImageOptimized  *ImageOptimization `json:"profileImageOptimized,omitzero"`
	IsCloseOnly            bool               `json:"isCloseOnly,omitzero"`
	IsCertReq              bool               `json:"isCertReq,omitzero"`
	CertReqDate            time.Time          `json:"certReqDate,omitzero"`
	DiscordUsername        string             `json:"discordUsername,omitzero"`
	XUsername              string             `json:"xUsername,omitzero"`
	VerifiedBadge          bool               `json:"verifiedBadge,omitzero"`
	DubPartnerID           string             `json:"dubPartnerId,omitzero"`
	TermsAcceptedAt        *time.Time         `json:"termsAcceptedAt,omitzero"`
	ViewOnlyAcknowledgedAt *time.Time         `json:"viewOnlyAcknowledgedAt,omitzero"`
	IsReferralRestricted   bool               `json:"isReferralRestricted,omitzero"`
}

// SearchResults is the structured response from Gamma public search.
type SearchResults struct {
	Events     []Event     `json:"events,omitzero"`
	Tags       []SearchTag `json:"tags,omitzero"`
	Profiles   []Profile   `json:"profiles,omitzero"`
	Pagination *Pagination `json:"pagination,omitzero"`
}

// SearchSort is the sort field for Gamma public search.
type SearchSort string

const (
	SearchSortVolume      SearchSort = "volume"
	SearchSortVolume24h   SearchSort = "volume_24hr"
	SearchSortLiquidity   SearchSort = "liquidity"
	SearchSortCompetitive SearchSort = "competitive"
	SearchSortClosedTime  SearchSort = "closed_time"
	SearchSortStartDate   SearchSort = "start_date"
	SearchSortEndDate     SearchSort = "end_date"
)

// validSearchSorts is the set of valid SearchSort values.
var validSearchSorts = map[SearchSort]struct{}{
	SearchSortVolume:      {},
	SearchSortVolume24h:   {},
	SearchSortLiquidity:   {},
	SearchSortCompetitive: {},
	SearchSortClosedTime:  {},
	SearchSortStartDate:   {},
	SearchSortEndDate:     {},
}

// IsValid returns true if s is a valid SearchSort.
func (s SearchSort) IsValid() bool {
	_, ok := validSearchSorts[s]
	return ok
}
