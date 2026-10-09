package gamma

// Comment is a thread row; root comments have a nil ParentCommentID.
type Comment struct {
	ID               string          `json:"id"`
	Body             *string         `json:"body,omitzero"`
	ReplyAddress     *string         `json:"replyAddress,omitzero"`
	UserAddress      *string         `json:"userAddress,omitzero"`
	ParentID         *string         `json:"parentId,omitzero"`
	CreatedAt        Timestamp       `json:"createdAt,omitzero"`
	UpdatedAt        Timestamp       `json:"updatedAt,omitzero"`
	Profile          *CommentProfile `json:"profile,omitzero"`
	Reactions        []Reaction      `json:"reactions,omitzero"`
	ReportCount      *int            `json:"reportCount,omitzero"`
	ReactionCount    *int            `json:"reactionCount,omitzero"`
	ParentCommentID  *FlexibleID     `json:"parentCommentID,omitzero"`
	ParentEntityType *string         `json:"parentEntityType,omitzero"`
	ParentEntityID   FlexibleID      `json:"parentEntityID,omitzero"`
	Media            []CommentMedia  `json:"media,omitzero"`
	TradeAsset       *string         `json:"tradeAsset,omitzero"`
}

// CommentProfile contains author information for a comment.
type CommentProfile struct {
	Name                  *string            `json:"name,omitzero"`
	Pseudonym             *string            `json:"pseudonym,omitzero"`
	DisplayUsernamePublic *bool              `json:"displayUsernamePublic,omitzero"`
	Bio                   *string            `json:"bio,omitzero"`
	IsMod                 *bool              `json:"isMod,omitzero"`
	IsCreator             *bool              `json:"isCreator,omitzero"`
	ProxyWallet           *string            `json:"proxyWallet,omitzero"`
	BaseAddress           *string            `json:"baseAddress,omitzero"`
	ProfileImage          *string            `json:"profileImage,omitzero"`
	Positions             []CommentPosition  `json:"positions,omitzero"`
	ProfileImageOptimized *ImageOptimization `json:"profileImageOptimized,omitzero"`
}

// CommentPosition represents a user's position relative to a comment.
type CommentPosition struct {
	TokenID      *FlexibleID `json:"tokenId,omitzero"`
	PositionSize Decimal     `json:"positionSize,omitzero"`
}

// Reaction represents a reaction to a comment.
type Reaction struct {
	ID           string          `json:"id"`
	CommentID    *int            `json:"commentID,omitzero"`
	ReactionType *string         `json:"reactionType,omitzero"`
	Icon         *string         `json:"icon,omitzero"`
	UserAddress  *string         `json:"userAddress,omitzero"`
	CreatedAt    Timestamp       `json:"createdAt,omitzero"`
	Profile      *CommentProfile `json:"profile,omitzero"`
}

// CommentMedia contains Gamma CommentMedia metadata.
type CommentMedia struct {
	ID              *string   `json:"id,omitzero"`
	CommentID       *int      `json:"commentID,omitzero"`
	Provider        *string   `json:"provider,omitzero"`
	ProviderMediaID *string   `json:"providerMediaId,omitzero"`
	URL             *string   `json:"url,omitzero"`
	MediaType       *string   `json:"mediaType,omitzero"`
	AltText         *string   `json:"altText,omitzero"`
	CreatedAt       Timestamp `json:"createdAt,omitzero"`
}
