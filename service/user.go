package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/util/common"
)

type UserService struct {
}

const (
	defaultAdminUsername = "admin"
	defaultAdminPassword = "admin"
)

type LoginResult struct {
	Username                string `json:"username"`
	DefaultCredentialsInUse bool   `json:"defaultCredentialsInUse"`
}

func IsDefaultAdminUsername(username string) bool {
	return strings.EqualFold(strings.TrimSpace(username), defaultAdminUsername)
}

func IsDefaultAdminCredentials(username string, password string) bool {
	return IsDefaultAdminUsername(username) && password == defaultAdminPassword
}

func GenerateAdminPassword() string {
	return common.Random(24)
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return common.NewError("username can not be empty")
	} else if password == "" {
		return common.NewError("password can not be empty")
	}
	hashedPassword, err := common.HashPassword(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	user := &model.User{}
	err = db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashedPassword
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	return db.Save(user).Error
}

func (s *UserService) Login(username string, password string, remoteIP string) (*LoginResult, error) {
	user := s.CheckUser(username, password, remoteIP)
	if user == nil {
		return nil, common.NewError("wrong user or password! IP: ", remoteIP)
	}
	return &LoginResult{
		Username:                user.Username,
		DefaultCredentialsInUse: IsDefaultAdminCredentials(user.Username, password),
	}, nil
}

func (s *UserService) CheckUser(username string, password string, remoteIP string) *model.User {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if database.IsNotFound(err) {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err, " IP: ", remoteIP)
		return nil
	}
	if !common.CheckPassword(user.Password, password) {
		return nil
	}
	if !common.IsPasswordHash(user.Password) {
		hashedPassword, err := common.HashPassword(password)
		if err == nil {
			err = db.Model(model.User{}).Where("id = ?", user.Id).Update("password", hashedPassword).Error
			if err != nil {
				logger.Warning("unable to migrate password hash", err)
			}
		} else {
			logger.Warning("unable to hash password", err)
		}
	}

	lastLoginTxt := time.Now().Format("2006-01-02 15:04:05") + " " + remoteIP
	err = db.Model(model.User{}).
		Where("username = ?", username).
		Update("last_logins", &lastLoginTxt).Error
	if err != nil {
		logger.Warning("unable to log login data", err)
	}
	return user
}

func (s *UserService) GetUsers() (*[]model.User, error) {
	var users []model.User
	db := database.GetDB()
	err := db.Model(model.User{}).Select("id,username,last_logins").Scan(&users).Error
	if err != nil {
		return nil, err
	}
	return &users, nil
}

func (s *UserService) ChangePass(id string, oldPass string, newUser string, newPass string) error {
	newUser = strings.TrimSpace(newUser)
	if newUser == "" {
		return common.NewError("username can not be empty")
	} else if newPass == "" {
		return common.NewError("password can not be empty")
	}
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).Where("id = ?", id).First(user).Error
	if err != nil || database.IsNotFound(err) {
		return err
	}
	if !common.CheckPassword(user.Password, oldPass) {
		return common.NewError("wrong password")
	}
	hashedPassword, err := common.HashPassword(newPass)
	if err != nil {
		return err
	}
	user.Username = newUser
	user.Password = hashedPassword
	return db.Save(user).Error
}

func (s *UserService) LoadTokens() ([]byte, error) {
	db := database.GetDB()
	var tokens []model.Tokens
	err := db.Model(model.Tokens{}).Preload("User").Where("expiry == 0 or expiry > ?", time.Now().Unix()).Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, t := range tokens {
		if !common.IsTokenHash(t.Token) {
			hashedToken := common.HashToken(t.Token)
			err = db.Model(model.Tokens{}).Where("id = ?", t.Id).Update("token", hashedToken).Error
			if err != nil {
				logger.Warning("unable to migrate token hash", err)
			} else {
				t.Token = hashedToken
			}
		}
		result = append(result, map[string]interface{}{
			"token":    t.Token,
			"expiry":   t.Expiry,
			"username": t.User.Username,
		})
	}
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return jsonResult, nil
}

func (s *UserService) GetUserTokens(username string) (*[]model.Tokens, error) {
	db := database.GetDB()
	var token []model.Tokens
	err := db.Model(model.Tokens{}).Select("id,desc,'****' as token,expiry,user_id").Where("user_id = (select id from users where username = ?)", username).Find(&token).Error
	if err != nil && !database.IsNotFound(err) {
		println(err.Error())
		return nil, err
	}
	return &token, nil
}

func (s *UserService) AddToken(username string, expiry int64, desc string) (string, error) {
	db := database.GetDB()
	var userId uint
	err := db.Model(model.User{}).Where("username = ?", username).Select("id").Scan(&userId).Error
	if err != nil {
		return "", err
	}
	if expiry > 0 {
		expiry = expiry*86400 + time.Now().Unix()
	}
	plainToken := common.Random(32)
	token := &model.Tokens{
		Token:  common.HashToken(plainToken),
		Desc:   desc,
		Expiry: expiry,
		UserId: userId,
	}
	err = db.Create(token).Error
	if err != nil {
		return "", err
	}
	return plainToken, nil
}

func (s *UserService) DeleteToken(id string) error {
	db := database.GetDB()
	return db.Model(model.Tokens{}).Where("id = ?", id).Delete(&model.Tokens{}).Error
}
