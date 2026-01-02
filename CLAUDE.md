# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Windows 自启动设置工具 - A simple console-based tool to manage Windows startup programs by modifying the registry key `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`.

## Commands

```bash
go mod tidy          # Install dependencies
go run .             # Run the application directly
go build -o autostart.exe  # Compile to executable
```

## Architecture

Single-file Go application (`main.go` ~600 lines) with no external GUI frameworks.

**Core Components:**
- **Menu System** (`showMainMenu`): Console-based main menu loop (options 1-4)
- **Registry Operations**: `AddToStartup`, `RemoveFromStartup`, `IsInStartup` - manipulate `Run` registry key
- **File Browser**: `selectExeFile` + `browseDirectory` - traverses directories to find .exe files
- **Windows Integration**: Uses `user32.dll` MessageBoxW for native dialogs

**Key Functions:**
- `handleAddToStartup()`: Prompts user to select .exe, confirms, then adds to registry
- `handleRemoveFromStartup()`: Lists current startup items, allows selection and removal
- `showStartupStatus()`: Displays all configured startup programs

## Technical Details

- Go 1.21+
- Dependency: `golang.org/x/sys` for Windows registry and native API access
- Path handling: Uses `filepath.Abs` for canonical paths, wraps paths in quotes for registry
- Chinese text output support (GBK encoding handled by Windows console)
