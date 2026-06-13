package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/service"
	"github.com/yongwei9527-art/s-ui-go/util"
)

type SubService struct {
	service.SettingService
	LinkService
}

func (s *SubService) GetSubs(subId string, hostname string) (*string, []string, error) {
	var err error

	client, err := s.getClientBySubId(subId)
	if err != nil {
		return nil, nil, err
	}

	clientInfo := ""
	subShowInfo, _ := s.SettingService.GetSubShowInfo()
	if subShowInfo {
		clientInfo = s.getClientInfo(client)
	}

	linksArray, err := s.getLocalLinks(client, hostname, clientInfo)
	if err != nil {
		return nil, nil, err
	}
	linksArray = append(linksArray, s.LinkService.GetLinks(&client.Links, "external", "")...)
	result := strings.Join(linksArray, "\n")

	headers := s.getClientHeaders(client)

	subEncode, _ := s.SettingService.GetSubEncode()
	if subEncode {
		result = base64.StdEncoding.EncodeToString([]byte(result))
	}

	return &result, headers, nil
}

func (s *SubService) getLocalLinks(client *model.Client, hostname string, clientInfo string) ([]string, error) {
	var inboundIDs []uint
	if err := json.Unmarshal(client.Inbounds, &inboundIDs); err != nil {
		return nil, err
	}
	if len(inboundIDs) == 0 {
		return nil, nil
	}

	db := database.GetDB()
	inbounds := make([]model.Inbound, 0)
	if err := db.Model(model.Inbound{}).Preload("Tls").Where("id in ? and type in ?", inboundIDs, util.InboundTypeWithLink).Find(&inbounds).Error; err != nil {
		return nil, err
	}

	links := make([]string, 0)
	for _, inbound := range inbounds {
		for _, link := range util.LinkGenerator(client.Config, &inbound, hostname) {
			links = append(links, s.addClientInfo(link, clientInfo))
		}
	}
	return links, nil
}

func (j *SubService) getClientBySubId(subId string) (*model.Client, error) {
	db := database.GetDB()
	client := &model.Client{}
	err := db.Model(model.Client{}).Where("enable = true and name = ?", subId).First(client).Error
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (s *SubService) getClientHeaders(client *model.Client) []string {
	updateInterval, _ := s.SettingService.GetSubUpdates()
	return util.GetHeaders(client, updateInterval)
}

func (s *SubService) getClientInfo(c *model.Client) string {
	now := time.Now().Unix()

	var result []string
	if vol := c.Volume - (c.Up + c.Down); vol > 0 {
		result = append(result, fmt.Sprintf("%s%s", s.formatTraffic(vol), "📊"))
	}
	if c.Expiry > 0 {
		result = append(result, fmt.Sprintf("%d%s⏳", (c.Expiry-now)/86400, "Days"))
	}
	if len(result) > 0 {
		return " " + strings.Join(result, " ")
	} else {
		return " ♾"
	}
}

func (s *SubService) formatTraffic(trafficBytes int64) string {
	if trafficBytes < 1024 {
		return fmt.Sprintf("%.2fB", float64(trafficBytes)/float64(1))
	} else if trafficBytes < (1024 * 1024) {
		return fmt.Sprintf("%.2fKB", float64(trafficBytes)/float64(1024))
	} else if trafficBytes < (1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fMB", float64(trafficBytes)/float64(1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fGB", float64(trafficBytes)/float64(1024*1024*1024))
	} else if trafficBytes < (1024 * 1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fTB", float64(trafficBytes)/float64(1024*1024*1024*1024))
	} else {
		return fmt.Sprintf("%.2fEB", float64(trafficBytes)/float64(1024*1024*1024*1024*1024))
	}
}
