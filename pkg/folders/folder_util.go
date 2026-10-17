package folders

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchFolder 监控文件夹文件变化，当有新文件创建时触发回调
// 参数：
//   - folderPath: 要监控的文件夹路径
//   - onNewFile: 当发现新文件时的回调函数，参数为新文件的完整路径
//   - ctx: 用于取消监控的 context
//
// 返回：
//   - error: 初始化监控失败时返回错误
func WatchFolder(ctx context.Context, folderPath string, onNewFile func(filePath string)) error {
	// 检查文件夹是否存在
	if !FolderExists(folderPath) {
		return os.ErrNotExist
	}

	// 创建 watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// 启动 goroutine 处理事件
	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// 监听创建和写入事件（有时文件创建分为创建和写入两步）
				if event.Op&fsnotify.Create == fsnotify.Create {
					// 确保是文件而不是目录
					info, err := os.Stat(event.Name)
					if err == nil && !info.IsDir() {
						onNewFile(event.Name)
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	// 添加监控路径
	err = watcher.Add(folderPath)
	if err != nil {
		watcher.Close()
		return err
	}

	return nil
}

// CreateFolderIfNotExists 检查文件夹是否存在，如果不存在则创建它
func CreateFolderIfNotExists(folderPath string) error {
	// 检查路径是否已存在
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		// 文件夹不存在，创建它（包括所有必要的父目录）
		err = os.MkdirAll(folderPath, 0755)
		if err != nil {
			fmt.Println("创建文件夹失败:", err)
			return err
		}
	}
	return nil
}

// FolderExists 检查文件夹是否存在
func FolderExists(folderPath string) bool {
	info, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// EnsureFolderExists 确保文件夹存在，如果不存在则创建
// 与 CreateFolderIfNotExists 类似，但返回布尔值表示是否创建了新目录
func EnsureFolderExists(folderPath string) (bool, error) {
	exists := FolderExists(folderPath)
	if !exists {
		err := os.MkdirAll(folderPath, 0755)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// CreateFileFolder 创建文件所在目录（如果不存在）
func CreateFileFolder(filePath string) error {
	folderPath := filepath.Dir(filePath)
	return CreateFolderIfNotExists(folderPath)
}

// MoveFolder 移动文件夹下的文件夹和文件到目标目录下
func MoveFolder(path string, path2 string) error {
	// 检查源路径是否存在
	if !FolderExists(path) {
		return errors.New("检查源路径:不存在")
	}
	// 确保目标路径存在
	err := CreateFolderIfNotExists(path2)
	if err != nil {
		return errors.New("检查路径:不存在,创建失败")
	}

	// 读取源文件夹内容
	entries, err := os.ReadDir(path)
	if err != nil {
		return errors.New("读取源文件夹内容:失败")
	}
	// 遍历源文件夹中的所有文件和子文件夹
	for _, entry := range entries {
		sourcePath := filepath.Join(path, entry.Name())
		destPath := filepath.Join(path2, entry.Name())

		if entry.IsDir() {
			// 如果是子目录，递归移动
			MoveFolder(sourcePath, destPath)
			// 移动完成后删除原目录
			os.RemoveAll(sourcePath)
		} else {
			// 如果是文件，直接移动
			os.Rename(sourcePath, destPath)
		}
	}
	return nil
}

// CopyFolder 复制文件夹下的文件和子文件夹到目标目录下
func CopyFolder(path string, path2 string) error {
	// 检查源路径是否存在
	if !FolderExists(path) {
		return errors.New("检查源路径:不存在")
	}
	// 确保目标路径存在
	CreateFolderIfNotExists(path2)
	// 读取源文件夹内容
	entries, err := os.ReadDir(path)
	if err != nil {
		return errors.New("读取源文件夹内容:失败")
	}

	// 遍历源文件夹中的所有文件和子文件夹
	for _, entry := range entries {
		sourcePath := filepath.Join(path, entry.Name())
		destPath := filepath.Join(path2, entry.Name())

		if entry.IsDir() {
			// 如果是子目录，递归复制
			CopyFolder(sourcePath, destPath)
		} else {
			// 如果是文件，复制文件
			CopyFile(sourcePath, destPath)
		}
	}
	return nil
}

// copyFile 复制单个文件
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 获取源文件信息以保持权限
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// 创建目标文件
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// 复制文件内容
	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return err
	}

	// 设置目标文件权限与源文件相同
	return os.Chmod(dst, sourceInfo.Mode())
}

// GetFolderNames 获取路径下的所有文件夹名字
func GetFolderNames(path string) ([]string, error) {
	var folderNames []string

	// 检查路径是否存在
	if !FolderExists(path) {
		return folderNames, os.ErrNotExist
	}

	// 读取目录内容
	entries, err := os.ReadDir(path)
	if err != nil {
		return folderNames, err
	}

	// 遍历目录内容，只获取文件夹名字
	for _, entry := range entries {
		if entry.IsDir() {
			folderNames = append(folderNames, entry.Name())
		}
	}

	return folderNames, nil
}

// FileInfo 表示文件信息
type FileInfo struct {
	FullPath   string // 全路径
	FolderName string // 上方文件夹
	FileName   string // 文件名
	FileType   string // 文件类型（扩展名）
}

// GetFileNames 获取路径下的全部文件信息：全路径、文件名、类型
func GetFileNames(path string) ([]FileInfo, error) {
	var fileInfos []FileInfo

	// 检查路径是否存在
	if !FolderExists(path) {
		return fileInfos, os.ErrNotExist
	}

	// 读取目录内容
	entries, err := os.ReadDir(path)
	if err != nil {
		return fileInfos, err
	}

	// 遍历目录内容，只获取文件信息
	for _, entry := range entries {
		if !entry.IsDir() { // 只处理文件，排除目录
			fullPath := filepath.Join(path, entry.Name())
			fileName := entry.Name()
			ext := filepath.Ext(fileName)
			// 使用 filepath.Base 获取直接父目录的名字
			folderPath := filepath.Dir(fullPath)    // 获取完整父路径
			folderName := filepath.Base(folderPath) // 获取父路径的最后部分（即文件夹名）
			fileInfo := FileInfo{
				FullPath:   fullPath,
				FolderName: folderName,
				FileName:   fileName,
				FileType:   ext,
			}
			fileInfos = append(fileInfos, fileInfo)
		}
	}

	return fileInfos, nil
}

// GetAllFileNames 递归获取路径下所有文件信息，包括子目录中的文件
func GetAllFileNames(path string) ([]FileInfo, error) {
	var fileInfos []FileInfo

	// 检查路径是否存在
	if !FolderExists(path) {
		return fileInfos, os.ErrNotExist
	}
	preDir := ""
	// 使用 Walk 遍历目录树
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 检查是否为文件（不是目录）
		if !info.IsDir() {
			fileName := info.Name()
			ext := strings.ReplaceAll(filepath.Ext(fileName), ".", "")
			fileName = strings.ReplaceAll(fileName, "."+ext, "")
			// 获取文件所在目录
			fileInfo := FileInfo{
				FullPath:   filePath,
				FileName:   fileName,
				FileType:   ext,
				FolderName: preDir,
			}
			fileInfos = append(fileInfos, fileInfo)
		} else {
			preDir = info.Name()
		}

		return nil
	})

	if err != nil {
		return fileInfos, err
	}

	return fileInfos, nil
}

func DeleteFile(path string) error {
	// 根据路径删除文件
	// 先尝试正常删除
	err := os.Remove(path)
	if err != nil {
		// 如果是Windows系统，可能因为文件被占用而删除失败，尝试强制删除
		// 休眠一段时间后再次尝试删除
		time.Sleep(100 * time.Millisecond)
		err = os.Remove(path)
	}
	return err
}

// RenameFileName 仅重命名文件名，保持在同一目录
func RenameFileName(filePath string, newFileName string) error {
	dir := filepath.Dir(filePath)
	newFilePath := filepath.Join(dir, newFileName)
	return os.Rename(filePath, newFilePath)
}

func RenameFolderName(path string, name string) error {
	// 重新给文件夹命名
	// 获取文件夹所在目录
	dir := filepath.Dir(path)
	// 构建新路径
	newPath := filepath.Join(dir, name)
	// 使用 os.Rename 重命名文件夹
	return os.Rename(path, newPath)
}

// MigrateFolderData 将指定路径的文件夹内的数据迁移到新目录下，并删除原指定路径文件夹
func MigrateFolderData(sourcePath string, targetPath string) error {
	// 检查源路径是否存在
	if !FolderExists(sourcePath) {
		return os.ErrNotExist
	}
	// 确保目标路径存在
	err := CreateFolderIfNotExists(targetPath)
	if err != nil {
		return err
	}
	// 使用 MoveFolder 函数移动文件夹内容
	MoveFolder(sourcePath, targetPath)
	// 删除原文件夹
	err = os.RemoveAll(sourcePath)
	if err != nil {
		return err
	}

	return nil
}

func WriteFileContent(path string, content string) error {
	// 打开文件以写入模式
	// 覆盖原内容
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入内容
	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

// MoveFileToFolder 将文件移动到指定文件夹下
func MoveFileToFolder(filePath string, targetFolder string) error {
	// 检查源文件是否存在
	if !FileExists(filePath) {
		return errors.New("源文件不存在")
	}

	// 确保目标文件夹存在
	err := CreateFolderIfNotExists(targetFolder)
	if err != nil {
		return fmt.Errorf("创建目标文件夹失败: %w", err)
	}

	// 获取文件名
	fileName := filepath.Base(filePath)
	// 构造目标文件路径
	targetFilePath := filepath.Join(targetFolder, fileName)

	// 移动文件
	return os.Rename(filePath, targetFilePath)
}

// FileExists 检查文件是否存在（可能需要添加到现有代码中）
func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// CreateSubFolder 在指定文件夹下创建子文件夹
func CreateSubFolder(parentFolder, subFolderName string) error {
	subFolderPath := filepath.Join(parentFolder, subFolderName)
	err := CreateFolderIfNotExists(subFolderPath)
	return err
}

// CreateJoinFolder 拼接文件夹路径
func CreateJoinFolder(parentFolder, subFolderName string) string {
	newPath := filepath.Join(parentFolder, subFolderName)
	CreateFolderIfNotExists(newPath)
	return newPath
}

// 在只能文件夹下创建文件

// CreateFileInFolder 在指定文件夹中创建文件
func CreateFileInFolder(folderPath, fileName, content string) error {
	// 确保文件夹存在
	if err := CreateFolderIfNotExists(folderPath); err != nil {
		return fmt.Errorf("创建文件夹失败: %w", err)
	}

	// 构造完整的文件路径
	filePath := filepath.Join(folderPath, fileName)

	// 创建并写入文件
	return WriteFileContent(filePath, content)
}

// CreateEmptyFileInFolder 在指定文件夹中创建空文件
func CreateEmptyFileInFolder(folderPath, fileName string) error {
	// 确保文件夹存在
	if err := CreateFolderIfNotExists(folderPath); err != nil {
		return fmt.Errorf("创建文件夹失败: %w", err)
	}

	// 构造完整的文件路径
	filePath := filepath.Join(folderPath, fileName)

	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return nil
}

// CopyFileToFolderWithCustomName 将文件复制到指定文件夹下，支持自定义文件名
func CopyFileToFolderWithCustomName(filePath string, targetFolder string, customFileName ...string) error {
	// 检查源文件是否存在
	if !FileExists(filePath) {
		fmt.Println(filePath)
		return errors.New("源文件不存在")
	}

	// 确保目标文件夹存在
	err := CreateFolderIfNotExists(targetFolder)
	if err != nil {
		return fmt.Errorf("创建目标文件夹失败: %w", err)
	}

	var finalFileName string
	if len(customFileName) > 0 && customFileName[0] != "" {
		// 获取源文件扩展名
		sourceExt := filepath.Ext(filePath)

		// 检查自定义文件名是否已有扩展名
		customExt := filepath.Ext(customFileName[0])

		if customExt == "" {
			// 如果自定义名称没有扩展名，则添加源文件的扩展名
			finalFileName = customFileName[0] + sourceExt
		} else {
			// 如果自定义名称已有扩展名，则直接使用
			finalFileName = customFileName[0]
		}
	} else {
		// 否则使用源文件名
		finalFileName = filepath.Base(filePath)
	}

	// 构造目标文件路径
	targetFilePath := filepath.Join(targetFolder, finalFileName)

	// 复制文件（使用已有的 CopyFile 函数）
	return CopyFile(filePath, targetFilePath)
}
