package cmd

import (
	"fmt"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/service"
)

func resetAdmin() error {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		return err
	}

	userService := service.UserService{}
	err = userService.UpdateFirstUser("admin", "admin")
	if err != nil {
		return fmt.Errorf("重置管理员账号密码失败: %w", err)
	}
	fmt.Println("重置管理员账号密码成功")
	return nil
}

func updateAdmin(username string, password string) error {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		return err
	}

	if username != "" || password != "" {
		userService := service.UserService{}
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			return fmt.Errorf("设置管理员账号密码失败: %w", err)
		}
		fmt.Println("设置管理员账号密码成功")
	}
	return nil
}

func showAdmin() error {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		return err
	}
	userService := service.UserService{}
	userModel, err := userService.GetFirstUser()
	if err != nil {
		return fmt.Errorf("获取当前管理员信息失败: %w", err)
	}
	username := userModel.Username
	if username == "" || userModel.Password == "" {
		fmt.Println("当前用户名或密码为空")
	}
	fmt.Println("当前管理员账号信息：")
	fmt.Println("\t用户名：\t", username)
	fmt.Println("\t密码：\t", "<已隐藏>")
	return nil
}
