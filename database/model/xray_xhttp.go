package model

import "time"

// XrayXHTTPInbound is deliberately separate from Inbound: the embedded
// sing-box core cannot parse XHTTP, so these records are rendered for the
// embedded Xray runtime only.
type XrayXHTTPInbound struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Enable      bool   `json:"enable"`
	Tag         string `json:"tag" gorm:"uniqueIndex;not null"`
	Listen      string `json:"listen" gorm:"not null"`
	Port        uint16 `json:"port" gorm:"not null"`
	ServerName  string `json:"serverName" gorm:"not null"`
	Dest        string `json:"dest" gorm:"not null"`
	PrivateKey  string `json:"privateKey" gorm:"not null"`
	PublicKey   string `json:"publicKey" gorm:"not null"`
	ShortID     string `json:"shortId" gorm:"not null"`
	Fingerprint string `json:"fingerprint" gorm:"not null"`
	// Transport is either tcp (VLESS Reality + XTLS Vision) or xhttp.
	// Existing rows migrate to xhttp through normalizeXrayXHTTPInbound.
	Transport string    `json:"transport" gorm:"not null;default:xhttp"`
	Path      string    `json:"path" gorm:"not null"`
	Mode      string    `json:"mode" gorm:"not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// XrayXHTTPClient keeps the per-client UUID out of the global inbound and
// lets one S-UI subscription aggregate sing-box and embedded Xray links.
type XrayXHTTPClient struct {
	Id        uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	InboundID uint   `json:"inboundId" gorm:"not null;uniqueIndex:idx_xray_xhttp_inbound_client"`
	ClientID  uint   `json:"clientId" gorm:"not null;index;uniqueIndex:idx_xray_xhttp_inbound_client"`
	Enable    bool   `json:"enable" gorm:"not null;default:true"`
	UUID      string `json:"uuid" gorm:"not null"`
}
