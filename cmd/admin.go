package cmd

import (
	"fmt"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/service"
)

func resetAdmin() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	userService := service.UserService{}
	err = userService.UpdateFirstUser("admin", "admin")
	if err != nil {
		fmt.Println("重置管理员账号密码失败：", err)
	} else {
		fmt.Println("重置管理员账号密码成功")
	}
}

func updateAdmin(username string, password string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	if username != "" || password != "" {
		userService := service.UserService{}
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("设置管理员账号密码失败：", err)
		} else {
			fmt.Println("设置管理员账号密码成功")
		}
	}
}

func showAdmin() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	userService := service.UserService{}
	userModel, err := userService.GetFirstUser()
	if err != nil {
		fmt.Println("获取当前管理员信息失败：", err)
		return
	}
	username := userModel.Username
	if username == "" || userModel.Password == "" {
		fmt.Println("当前用户名或密码为空")
	}
	fmt.Println("当前管理员账号信息：")
	fmt.Println("\t用户名：\t", username)
	fmt.Println("\t密码：\t", "<已隐藏>")
}
