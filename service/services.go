package service

import (
	"encoding/json"

	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/util/common"

	"gorm.io/gorm"
)

type ServicesService struct{}

func (s *ServicesService) GetAll() (*[]map[string]interface{}, error) {
	db := database.GetDB()
	services := []model.Service{}
	err := db.Model(model.Service{}).Scan(&services).Error
	if err != nil {
		return nil, err
	}
	var data []map[string]interface{}
	for _, srv := range services {
		srvData := map[string]interface{}{
			"id":     srv.Id,
			"type":   srv.Type,
			"tag":    srv.Tag,
			"tls_id": srv.TlsId,
		}
		if srv.Options != nil {
			var restFields map[string]json.RawMessage
			if err := json.Unmarshal(srv.Options, &restFields); err != nil {
				return nil, err
			}
			for k, v := range restFields {
				srvData[k] = v
			}
		}

		data = append(data, srvData)
	}
	return &data, nil
}

func (s *ServicesService) GetAllConfig(db *gorm.DB) ([]json.RawMessage, error) {
	var servicesJson []json.RawMessage
	var services []*model.Service
	err := db.Model(model.Service{}).Preload("Tls").Find(&services).Error
	if err != nil {
		return nil, err
	}
	for _, srv := range services {
		srvJson, err := srv.MarshalJSON()
		if err != nil {
			return nil, err
		}
		servicesJson = append(servicesJson, srvJson)
	}
	return servicesJson, nil
}

func (s *ServicesService) Save(tx *gorm.DB, act string, data json.RawMessage) error {
	var err error

	switch act {
	case "new", "edit":
		var srv model.Service
		err = srv.UnmarshalJSON(data)
		if err != nil {
			return err
		}

		if srv.TlsId > 0 {
			err = tx.Model(model.Tls{}).Where("id = ?", srv.TlsId).Find(&srv.Tls).Error
			if err != nil {
				return err
			}
		}

		err = tx.Save(&srv).Error
		if err != nil {
			return err
		}
	case "del":
		var tag string
		err = json.Unmarshal(data, &tag)
		if err != nil {
			return err
		}
		err = tx.Where("tag = ?", tag).Delete(model.Service{}).Error
		if err != nil {
			return err
		}
	default:
		return common.NewErrorf("unknown action: %s", act)
	}
	return nil
}

func (s *ServicesService) RestartServices(tx *gorm.DB, ids []uint) error {
	// sing-box v1.14.3 removed dynamic component deletion. The caller now
	// restarts the complete core after the surrounding database transaction commits.
	return nil
}
