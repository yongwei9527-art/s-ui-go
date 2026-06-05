package cmd

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/service"

	"github.com/shirou/gopsutil/v4/net"
)

func resetSetting() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	settingService := service.SettingService{}
	err = settingService.ResetSettings()
	if err != nil {
		fmt.Println("重置设置失败：", err)
	} else {
		fmt.Println("重置设置成功")
	}
}

func updateSetting(port int, path string, subPort int, subPath string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	settingService := service.SettingService{}

	if port > 0 {
		err := settingService.SetPort(port)
		if err != nil {
			fmt.Println("设置面板端口失败：", err)
		} else {
			fmt.Println("设置面板端口成功")
		}
	}
	if path != "" {
		err := settingService.SetWebPath(path)
		if err != nil {
			fmt.Println("设置面板路径失败：", err)
		} else {
			fmt.Println("设置面板路径成功")
		}
	}
	if subPort > 0 {
		err := settingService.SetSubPort(subPort)
		if err != nil {
			fmt.Println("设置订阅端口失败：", err)
		} else {
			fmt.Println("设置订阅端口成功")
		}
	}
	if subPath != "" {
		err := settingService.SetSubPath(subPath)
		if err != nil {
			fmt.Println("设置订阅路径失败：", err)
		} else {
			fmt.Println("设置订阅路径成功")
		}
	}
}

func showSetting() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	settingService := service.SettingService{}
	allSetting, err := settingService.GetAllSetting()
	if err != nil {
		fmt.Println("获取当前设置失败：", err)
		return
	}
	fmt.Println("当前面板设置：")
	fmt.Println("\t面板端口：\t", (*allSetting)["webPort"])
	fmt.Println("\t面板路径：\t", (*allSetting)["webPath"])
	if (*allSetting)["webListen"] != "" {
		fmt.Println("\t监听 IP：\t", (*allSetting)["webListen"])
	}
	if (*allSetting)["webDomain"] != "" {
		fmt.Println("\t面板域名：\t", (*allSetting)["webDomain"])
	}
	if (*allSetting)["webURI"] != "" {
		fmt.Println("\t面板 URI：\t", (*allSetting)["webURI"])
	}
	fmt.Println()
	fmt.Println("当前订阅设置：")
	fmt.Println("\t订阅端口：\t", (*allSetting)["subPort"])
	fmt.Println("\t订阅路径：\t", (*allSetting)["subPath"])
	if (*allSetting)["subListen"] != "" {
		fmt.Println("\t订阅监听 IP：\t", (*allSetting)["subListen"])
	}
	if (*allSetting)["subDomain"] != "" {
		fmt.Println("\t订阅域名：\t", (*allSetting)["subDomain"])
	}
	if (*allSetting)["subURI"] != "" {
		fmt.Println("\t订阅 URI：\t", (*allSetting)["subURI"])
	}
}

func getPublicIP() string {
	apis := []string{
		"https://api64.ipify.org",
		"https://ip.sb",
		"https://icanhazip.com",
		"https://ipinfo.io/ip",
		"https://checkip.amazonaws.com",
	}
	type result struct {
		ip  string
		err error
	}
	ch := make(chan result, len(apis))
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 3 * time.Second}

	for _, api := range apis {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			resp, err := client.Get(url)
			if err != nil {
				ch <- result{"", err}
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				ch <- result{"", err}
				return
			}
			ch <- result{string(body), nil}
		}(api)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for res := range ch {
		if res.err == nil && res.ip != "" {
			return strings.TrimSpace(res.ip)
		}
	}
	return ""
}

func formatHostForURI(host string) string {
	host = strings.TrimSpace(host)
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

func getPanelURI() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	settingService := service.SettingService{}
	Port, _ := settingService.GetPort()
	BasePath, _ := settingService.GetWebPath()
	Listen, _ := settingService.GetListen()
	Domain, _ := settingService.GetWebDomain()
	KeyFile, _ := settingService.GetKeyFile()
	CertFile, _ := settingService.GetCertFile()
	TLS := false
	if KeyFile != "" && CertFile != "" {
		TLS = true
	}
	Proto := ""
	if TLS {
		Proto = "https://"
	} else {
		Proto = "http://"
	}
	PortText := fmt.Sprintf(":%d", Port)
	if (Port == 443 && TLS) || (Port == 80 && !TLS) {
		PortText = ""
	}
	if len(Domain) > 0 {
		fmt.Println(Proto + formatHostForURI(Domain) + PortText + BasePath)
		return
	}
	if len(Listen) > 0 {
		fmt.Println(Proto + formatHostForURI(Listen) + PortText + BasePath)
		return
	}
	fmt.Println("本机地址：")
	netInterfaces, _ := net.Interfaces()
	for i := 0; i < len(netInterfaces); i++ {
		if len(netInterfaces[i].Flags) > 2 && netInterfaces[i].Flags[0] == "up" && netInterfaces[i].Flags[1] != "loopback" {
			addrs := netInterfaces[i].Addrs
			for _, address := range addrs {
				IP := strings.Split(address.Addr, "/")[0]
				if strings.Contains(address.Addr, ".") {
					fmt.Println(Proto + IP + PortText + BasePath)
				} else if IP != "" && !strings.HasPrefix(strings.ToLower(IP), "fe80::") {
					fmt.Println(Proto + "[" + IP + "]" + PortText + BasePath)
				}
			}
		}
	}
	pubIP := getPublicIP()
	if pubIP != "" {
		fmt.Printf("\n公网完整地址：\n%s%s%s\n", Proto, formatHostForURI(pubIP), PortText+BasePath)
	}
}
