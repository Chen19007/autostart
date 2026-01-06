package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// 缓存文件路径
var cacheFilePath string

// 缓存数据结构
type CacheItem struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type CacheData struct {
	Items []CacheItem `json:"items"`
}

const (
	// 注册表路径：当前用户的启动项
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
)

func init() {
	// 初始化缓存文件路径
	exePath, _ := os.Executable()
	cacheFilePath = filepath.Join(filepath.Dir(exePath), "autostart.json")

	// 启动时同步缓存
	syncCacheFromRegistry()
}

// syncCacheFromRegistry 从注册表同步缓存
func syncCacheFromRegistry() {
	cache, err := loadCache()
	if err != nil {
		return
	}

	// 打开注册表
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	// 获取所有注册表项名称
	names, err := key.ReadValueNames(0)
	if err != nil {
		return
	}

	// 构建注册表项map，用于快速查找
	registryItems := make(map[string]string)
	for _, name := range names {
		value, _, err := key.GetStringValue(name)
		if err == nil {
			registryItems[name] = value
		}
	}

	// 步骤1：遍历缓存，设置 disable
	// 缓存中存在但注册表中不存在 → 标记为禁用
	for i := range cache.Items {
		item := &cache.Items[i]
		if _, exists := registryItems[item.Name]; !exists {
			item.Enabled = false
		}
	}

	// 步骤2：遍历注册表，设置 enable
	// 注册表中存在 → 添加到缓存或更新，并标记为启用
	for name, value := range registryItems {
		idx, _ := findItemByName(cache, name)
		if idx >= 0 {
			// 缓存中存在，更新值并标记为启用
			cache.Items[idx].Value = value
			cache.Items[idx].Enabled = true
		} else {
			// 缓存中不存在，添加到缓存并标记为启用
			cache.Items = append(cache.Items, CacheItem{
				Name:    name,
				Value:   value,
				Enabled: true,
			})
		}
	}

	// 保存缓存
	saveCache(cache)
}

func main() {
	// 显示主菜单
	showMainMenu()
}

// loadCache 加载缓存文件
func loadCache() (*CacheData, error) {
	data := &CacheData{}

	file, err := os.Open(cacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// saveCache 保存缓存文件
func saveCache(data *CacheData) error {
	file, err := os.Create(cacheFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// findItemByName 根据名称查找缓存项
func findItemByName(data *CacheData, name string) (int, *CacheItem) {
	for i, item := range data.Items {
		if item.Name == name {
			return i, &item
		}
	}
	return -1, nil
}

// addOrUpdateItem 添加或更新缓存项
func addOrUpdateItem(data *CacheData, name, value string, enabled bool) {
	idx, _ := findItemByName(data, name)
	if idx >= 0 {
		data.Items[idx].Value = value
		data.Items[idx].Enabled = enabled
	} else {
		data.Items = append(data.Items, CacheItem{
			Name:    name,
			Value:   value,
			Enabled: enabled,
		})
	}
}

// removeItem 从缓存中删除项
func removeItem(data *CacheData, name string) {
	idx, _ := findItemByName(data, name)
	if idx >= 0 {
		data.Items = append(data.Items[:idx], data.Items[idx+1:]...)
	}
}

// showMainMenu 显示主菜单
func showMainMenu() {
	for {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("        Windows 自启动设置工具")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Println("1. 添加程序到自启动")
		fmt.Println("2. 移除程序的自启动")
		fmt.Println("3. 查看当前自启动状态")
		fmt.Println("4. 添加命令到自启动")
		fmt.Println("5. 启用")
		fmt.Println("6. 禁用")
		fmt.Println("7. 退出")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Print("请选择操作 (1-7): ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			handleAddToStartup()
		case "2":
			handleRemoveFromStartup()
		case "3":
			showStartupStatus()
		case "4":
			handleAddCommand()
		case "5":
			handleEnable()
		case "6":
			handleDisable()
		case "7":
			fmt.Println("再见！")
			return
		default:
			fmt.Println("无效的选择，请重新输入。")
		}
	}
}

// handleAddToStartup 处理添加自启动
func handleAddToStartup() {
	exePath := selectExeFile()
	if exePath == "" {
		return // 用户取消了选择
	}

	// 获取程序名称作为注册表项名称
	appName := getAppName(exePath)

	// 获取绝对路径并构建注册表值
	absPath, _ := filepath.Abs(exePath)
	regValue := fmt.Sprintf(`"%s"`, absPath)

	// 检查是否已经在注册表中
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err == nil {
		_, _, err = key.GetStringValue(appName)
		key.Close()
	}

	exists := err == nil

	if exists {
		fmt.Printf("\n程序 %s 已经在自启动列表中。\n", appName)
		fmt.Print("是否要重新设置？(y/n): ")
		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm != "y" && confirm != "yes" {
			return
		}
	}

	// 确认添加
	fmt.Printf("\n确定要将以下程序添加到自启动吗？\n")
	fmt.Printf("程序路径: %s\n", exePath)
	fmt.Printf("程序名称: %s\n", appName)
	fmt.Print("确认添加？(y/n): ")
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm == "y" || confirm == "yes" {
		// 添加到注册表
		err := AddToStartup(exePath, appName)
		if err != nil {
			fmt.Printf("添加失败: %v\n", err)
			fmt.Printf("\n错误: 添加失败 - %v\n", err)
		} else {
			// 更新缓存
			cache, _ := loadCache()
			addOrUpdateItem(cache, appName, regValue, true)
			saveCache(cache)

			fmt.Printf("已成功将 %s 添加到自启动！\n", appName)
		}
	}
}

// handleRemoveFromStartup 处理移除自启动
func handleRemoveFromStartup() {
	// 获取所有自启动项
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		fmt.Printf("无法读取注册表: %v\n", err)
		return
	}
	defer key.Close()

	// 获取所有值名称
	names, err := key.ReadValueNames(0)
	if err != nil {
		fmt.Printf("读取注册表值失败: %v\n", err)
		return
	}

	if len(names) == 0 {
		fmt.Println("当前没有设置任何自启动程序。")
		return
	}

	// 显示自启动项列表
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("当前自启动程序列表：")
	fmt.Println(strings.Repeat("=", 60))

	type startupItem struct {
		name  string
		value string
		index int
	}

	items := make([]startupItem, 0, len(names))
	for i, name := range names {
		value, _, err := key.GetStringValue(name)
		if err == nil {
			items = append(items, startupItem{
				name:  name,
				value: value,
				index: i + 1,
			})
			fmt.Printf("%d. %s\n   %s\n\n", i+1, name, value)
		} else {
			items = append(items, startupItem{
				name:  name,
				value: "(无法读取路径)",
				index: i + 1,
			})
			fmt.Printf("%d. %s\n   (无法读取路径)\n\n", i+1, name)
		}
	}

	// 让用户选择要移除的项
	fmt.Println(strings.Repeat("=", 60))
	fmt.Print("请输入要移除的项编号（或输入 'b' 返回）: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "b" || choice == "B" {
		return
	}

	// 解析序号
	num, err := strconv.Atoi(choice)
	if err != nil {
		fmt.Println("无效的编号。")
		return
	}

	if num < 1 || num > len(items) {
		fmt.Println("无效的编号。")
		return
	}

	selectedItem := items[num-1]

	// 确认移除
	fmt.Printf("\n确定要从自启动中移除以下程序吗？\n")
	fmt.Printf("程序名称: %s\n", selectedItem.name)
	fmt.Printf("程序路径: %s\n", selectedItem.value)
	fmt.Print("确认移除？(y/n): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm == "y" || confirm == "yes" {
		err := RemoveFromStartup(selectedItem.name)
		if err != nil {
			fmt.Printf("\n错误: 移除失败 - %v\n", err)
		} else {
			// 从缓存中删除
			cache, _ := loadCache()
			removeItem(cache, selectedItem.name)
			saveCache(cache)

			fmt.Printf("已成功从自启动中移除 %s！\n", selectedItem.name)
		}
	}
}

// showStartupStatus 显示当前自启动状态
func showStartupStatus() {
	cache, err := loadCache()
	if err != nil {
		fmt.Printf("加载缓存失败: %v\n", err)
		return
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("当前自启动程序列表：")
	fmt.Println(strings.Repeat("=", 60))

	if len(cache.Items) == 0 {
		fmt.Println("当前没有配置任何自启动程序。")
		return
	}

	// 排序显示
	sort.Slice(cache.Items, func(i, j int) bool {
		return cache.Items[i].Name < cache.Items[j].Name
	})

	for i, item := range cache.Items {
		status := "[启用]"
		if !item.Enabled {
			status = "[禁用]"
		}
		fmt.Printf("%d. %s %s\n   %s\n\n", i+1, item.Name, status, item.Value)
	}
}

// selectExeFile 选择exe文件
func selectExeFile() string {
	currentDir, _ := os.Getwd()

	for {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("选择程序文件")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("当前目录: %s\n", currentDir)
		fmt.Print("请输入文件路径（或输入 'b' 返回，输入 'd' 浏览当前目录，输入 'g' 跳转到指定目录）: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "b" || input == "B" {
			return ""
		}

		if input == "d" || input == "D" {
			// 浏览当前目录
			selected := browseDirectory(currentDir)
			if selected != "" {
				return selected
			}
			continue
		}

		if input == "g" || input == "G" {
			// 跳转到指定目录
			fmt.Print("请输入要跳转的目录路径: ")
			newPath, _ := reader.ReadString('\n')
			newPath = strings.TrimSpace(newPath)

			if newPath == "" {
				continue
			}

			// 处理路径
			var targetDir string
			if filepath.IsAbs(newPath) {
				targetDir = newPath
			} else {
				// 相对路径，相对于当前目录
				targetDir = filepath.Join(currentDir, newPath)
			}

			// 检查是否是有效目录
			info, err := os.Stat(targetDir)
			if err != nil {
				fmt.Printf("目录不存在或无法访问: %v\n", err)
				fmt.Print("按回车键继续...")
				reader.ReadString('\n')
				continue
			}

			if !info.IsDir() {
				fmt.Printf("路径不是目录: %s\n", targetDir)
				fmt.Print("按回车键继续...")
				reader.ReadString('\n')
				continue
			}

			// 更新当前目录
			absTarget, _ := filepath.Abs(targetDir)
			currentDir = absTarget
			continue
		}

		// 检查是否是完整路径
		if filepath.IsAbs(input) {
			if isValidExeFile(input) {
				return input
			}
			fmt.Printf("文件不存在或不是有效的exe文件: %s\n", input)
			continue
		}

		// 尝试作为相对路径
		absPath, err := filepath.Abs(input)
		if err == nil && isValidExeFile(absPath) {
			return absPath
		}

		fmt.Printf("文件不存在或不是有效的exe文件: %s\n", input)
	}
}

// browseDirectory 浏览目录（只一层）
func browseDirectory(dirPath string) string {
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		fmt.Printf("无法获取目录路径: %v\n", err)
		return ""
	}

	// 读取目录内容
	entries, err := os.ReadDir(absDir)
	if err != nil {
		fmt.Printf("无法读取目录: %v\n", err)
		return ""
	}

	// 过滤出exe文件和目录
	var exeFiles []string
	var dirs []string

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		} else if strings.ToLower(filepath.Ext(entry.Name())) == ".exe" {
			exeFiles = append(exeFiles, entry.Name())
		}
	}

	// 排序
	sort.Strings(dirs)
	sort.Strings(exeFiles)

	// 显示文件列表
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("当前目录: %s\n", absDir)
	fmt.Println(strings.Repeat("=", 60))

	index := 1
	fileMap := make(map[int]string)

	// 显示目录
	if len(dirs) > 0 {
		fmt.Println("\n目录:")
		for _, dir := range dirs {
			fmt.Printf("  [%d] %s/ (目录)\n", index, dir)
			fileMap[index] = filepath.Join(absDir, dir)
			index++
		}
	}

	// 显示exe文件
	if len(exeFiles) > 0 {
		fmt.Println("\n可执行文件:")
		for _, file := range exeFiles {
			fullPath := filepath.Join(absDir, file)
			fmt.Printf("  [%d] %s\n", index, file)
			fileMap[index] = fullPath
			index++
		}
	}

	if len(fileMap) == 0 {
		fmt.Println("当前目录没有找到exe文件或子目录。")
		return ""
	}

	// 让用户选择
	fmt.Println(strings.Repeat("=", 60))
	fmt.Print("请输入序号选择文件（或输入 'b' 返回，输入 'u' 返回上一级，输入 'g' 跳转到指定目录）: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "b" || choice == "B" {
		return ""
	}

	if choice == "u" || choice == "U" {
		parentDir := filepath.Dir(absDir)
		if parentDir != absDir {
			return browseDirectory(parentDir)
		}
		return browseDirectory(absDir)
	}

	if choice == "g" || choice == "G" {
		// 跳转到指定目录
		fmt.Print("请输入要跳转的目录路径: ")
		newPath, _ := reader.ReadString('\n')
		newPath = strings.TrimSpace(newPath)

		if newPath == "" {
			return browseDirectory(absDir)
		}

		// 处理路径
		var targetDir string
		if filepath.IsAbs(newPath) {
			targetDir = newPath
		} else {
			// 相对路径，相对于当前目录
			targetDir = filepath.Join(absDir, newPath)
		}

		// 检查是否是有效目录
		info, err := os.Stat(targetDir)
		if err != nil {
			fmt.Printf("目录不存在或无法访问: %v\n", err)
			fmt.Print("按回车键继续...")
			reader.ReadString('\n')
			return browseDirectory(absDir)
		}

		if !info.IsDir() {
			fmt.Printf("路径不是目录: %s\n", targetDir)
			fmt.Print("按回车键继续...")
			reader.ReadString('\n')
			return browseDirectory(absDir)
		}

		// 跳转到新目录
		return browseDirectory(targetDir)
	}

	// 解析序号
	num, err := strconv.Atoi(choice)
	if err != nil {
		fmt.Println("无效的序号。")
		return ""
	}

	selectedPath, exists := fileMap[num]
	if !exists {
		fmt.Println("无效的序号。")
		return ""
	}

	// 检查是否是目录
	info, err := os.Stat(selectedPath)
	if err != nil {
		fmt.Printf("无法访问: %v\n", err)
		return ""
	}

	if info.IsDir() {
		// 进入子目录
		return browseDirectory(selectedPath)
	}

	// 返回选中的exe文件
	return selectedPath
}

// isValidExeFile 检查是否是有效的exe文件
func isValidExeFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return strings.ToLower(filepath.Ext(path)) == ".exe"
}

// getAppName 从文件路径获取程序名称（不含扩展名）
func getAppName(exePath string) string {
	base := filepath.Base(exePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// AddToStartup 添加程序到Windows自启动
func AddToStartup(exePath, appName string) error {
	// 获取可执行文件的绝对路径
	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %v", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", absPath)
	}

	// 打开注册表键
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	// 设置注册表值（使用双引号包裹路径，防止路径中有空格）
	value := fmt.Sprintf(`"%s"`, absPath)
	err = key.SetStringValue(appName, value)
	if err != nil {
		return fmt.Errorf("设置注册表值失败: %v", err)
	}

	return nil
}

// RemoveFromStartup 从Windows自启动中移除程序
func RemoveFromStartup(appName string) error {
	// 打开注册表键
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	// 删除注册表值
	err = key.DeleteValue(appName)
	if err != nil {
		if err == registry.ErrNotExist {
			return fmt.Errorf("启动项不存在")
		}
		return fmt.Errorf("删除注册表值失败: %v", err)
	}

	return nil
}

// ========== 公共基础函数 ==========

// ListItem 列表项结构
type ListItem struct {
	Name  string
	Value string
}

// showListWithBack 显示列表，支持返回
func showListWithBack(items []ListItem, title string) (int, bool) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println(title)
	fmt.Println(strings.Repeat("=", 60))

	if len(items) == 0 {
		fmt.Println("没有可显示的项。")
		return -1, false
	}

	for i, item := range items {
		fmt.Printf("%d. %s\n   %s\n\n", i+1, item.Name, item.Value)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Print("请输入序号（或输入 'b' 返回）: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "b" || choice == "B" {
		return -1, false
	}

	num, err := strconv.Atoi(choice)
	if err != nil || num < 1 || num > len(items) {
		fmt.Println("无效的编号。")
		return -1, false
	}

	return num - 1, true
}

// confirmWithBack 确认操作，支持返回
// 返回：true=确认，false=取消或返回
func confirmWithBack(title, name, value string) bool {
	fmt.Printf("\n确定要%s吗？\n", title)
	fmt.Printf("名称: %s\n", name)
	fmt.Printf("内容: %s\n", value)
	fmt.Print("确认？(y/n/b 返回): ")

	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm == "b" || confirm == "B" {
		return false
	}
	return confirm == "y" || confirm == "yes"
}

// IsInStartup 检查程序是否已在自启动列表中
func IsInStartup(exePath, appName string) (bool, error) {
	// 打开注册表键
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	// 检查值是否存在
	value, _, err := key.GetStringValue(appName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("查询注册表值失败: %v", err)
	}

	// 检查路径是否匹配（去除引号）
	absPath, _ := filepath.Abs(exePath)
	value = strings.Trim(value, `"`)
	valueAbs, _ := filepath.Abs(value)

	return absPath == valueAbs, nil
}

// handleAddCommand 处理添加自定义命令
func handleAddCommand() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("添加自定义命令到自启动")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Print("（输入 'b' 返回主菜单）\n\n")

		// 步骤1：输入注册表项名称
		fmt.Print("请输入注册表项名称（如 TaskManager）: ")
		appName, _ := reader.ReadString('\n')
		appName = strings.TrimSpace(appName)

		if appName == "b" || appName == "B" {
			return
		}

		if appName == "" {
			fmt.Println("名称不能为空。")
			continue
		}

		// 步骤2：输入启动命令
		fmt.Println("\n请输入完整的启动命令：")
		fmt.Println("示例：python E:\\project\\python\\task-manager\\main.py")
		fmt.Println("示例：E:\\app\\program.exe --arg value")
		fmt.Print("（输入 'b' 返回上一步）\n\n命令: ")

		command, _ := reader.ReadString('\n')
		command = strings.TrimSpace(command)

		if command == "b" || command == "B" {
			continue
		}

		if command == "" {
			fmt.Println("命令不能为空。")
			continue
		}

		// 确认添加
		if confirmWithBack("添加", appName, command) {
			err := AddCommandToStartup(command, appName)
			if err != nil {
				fmt.Printf("\n错误: 添加失败 - %v\n", err)
			} else {
				// 更新缓存
				cache, _ := loadCache()
				addOrUpdateItem(cache, appName, command, true)
				saveCache(cache)

				fmt.Printf("已成功将命令添加到自启动！\n")
			}
		}

		// 添加成功后返回主菜单
		return
	}
}

// AddCommandToStartup 添加自定义命令到Windows自启动
func AddCommandToStartup(command, appName string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	err = key.SetStringValue(appName, command)
	if err != nil {
		return fmt.Errorf("设置注册表值失败: %v", err)
	}

	return nil
}

// IsCommandInStartup 检查注册表项是否已存在
func IsCommandInStartup(appName string) (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	if err == nil {
		return true, nil
	}
	if err == registry.ErrNotExist {
		return false, nil
	}
	return false, fmt.Errorf("查询注册表失败: %v", err)
}

// handleEnable 启用禁用的启动项
func handleEnable() {
	cache, err := loadCache()
	if err != nil {
		fmt.Printf("加载缓存失败: %v\n", err)
		return
	}

	// 筛选出禁用的项并转换为 ListItem
	var disabledItems []ListItem
	for _, item := range cache.Items {
		if !item.Enabled {
			disabledItems = append(disabledItems, ListItem{
				Name:  item.Name,
				Value: item.Value,
			})
		}
	}

	idx, ok := showListWithBack(disabledItems, "禁用的启动项列表")
	if !ok {
		return
	}

	selectedItem := disabledItems[idx]

	// 确认启用
	if !confirmWithBack("启用", selectedItem.Name, selectedItem.Value) {
		return
	}

	// 添加到注册表
	err = AddCommandToStartup(selectedItem.Value, selectedItem.Name)
	if err != nil {
		fmt.Printf("\n错误: 启用失败 - %v\n", err)
	} else {
		// 更新缓存
		addOrUpdateItem(cache, selectedItem.Name, selectedItem.Value, true)
		saveCache(cache)

		fmt.Printf("已成功启用 %s！\n", selectedItem.Name)
	}
}

// handleDisable 禁用启用的启动项
func handleDisable() {
	cache, err := loadCache()
	if err != nil {
		fmt.Printf("加载缓存失败: %v\n", err)
		return
	}

	// 筛选出启用的项并转换为 ListItem
	var enabledItems []ListItem
	for _, item := range cache.Items {
		if item.Enabled {
			enabledItems = append(enabledItems, ListItem{
				Name:  item.Name,
				Value: item.Value,
			})
		}
	}

	idx, ok := showListWithBack(enabledItems, "已启用的启动项列表")
	if !ok {
		return
	}

	selectedItem := enabledItems[idx]

	// 确认禁用
	if !confirmWithBack("禁用", selectedItem.Name, selectedItem.Value) {
		return
	}

	// 从注册表删除
	err = RemoveFromStartup(selectedItem.Name)
	if err != nil {
		fmt.Printf("\n错误: 禁用失败 - %v\n", err)
	} else {
		// 更新缓存
		addOrUpdateItem(cache, selectedItem.Name, selectedItem.Value, false)
		saveCache(cache)

		fmt.Printf("已成功禁用 %s！\n", selectedItem.Name)
	}
}
