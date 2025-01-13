package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type FileLocation struct {
	Path   string
	Line   int
	Column int
}

func parseFileLocation(line string) FileLocation {
	parts := strings.Split(line, ":")
	if len(parts) != 3 {
		return FileLocation{Path: line, Line: 1, Column: 1}
	}

	var loc FileLocation
	loc.Path = parts[0]
	fmt.Sscanf(parts[1], "%d", &loc.Line)
	fmt.Sscanf(parts[2], "%d", &loc.Column)
	return loc
}

func getGitFileContent(path string, line int, column int) string {
	cmd := exec.Command("git", "show", fmt.Sprintf("HEAD:%s", path))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("Error reading git file: %v", err)
	}
	return string(output)
}

func getWorkingSetContent(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}
	return string(content)
}

func loadFileLocations(path string) []FileLocation {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var locations []FileLocation
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		locations = append(locations, parseFileLocation(scanner.Text()))
	}
	return locations
}

func main() {
	myApp := app.New()
	window := myApp.NewWindow("ASQ GUI")

	var locations []FileLocation
	
	// Create the list for the top pane
	fileList := widget.NewList(
		func() int { return len(locations) },
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			label := item.(*widget.Label)
			loc := locations[id]
			label.SetText(fmt.Sprintf("%s:%d:%d", loc.Path, loc.Line, loc.Column))
		},
	)

	// Create text areas for the bottom panes
	gitCommitCode := widget.NewTextArea()
	gitCommitCode.Disable() // Read-only
	workingSetCode := widget.NewTextArea()
	workingSetCode.Disable() // Read-only

	// Create split containers
	bottomSplit := container.NewHSplit(
		container.NewScroll(gitCommitCode),
		container.NewScroll(workingSetCode),
	)
	bottomSplit.SetOffset(0.5) // Equal split

	mainSplit := container.NewVSplit(
		container.NewScroll(fileList),
		bottomSplit,
	)
	mainSplit.SetOffset(0.3) // 30% top, 70% bottom

	window.SetContent(mainSplit)
	window.Resize(fyne.NewSize(1024, 768))

	// Load initial file locations from a file
	if len(os.Args) > 1 {
		locations = loadFileLocations(os.Args[1])
		fileList.Refresh()
	}

	// Handle list selection
	fileList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(locations) {
			return
		}
		loc := locations[id]
		
		// Update git commit version
		gitContent := getGitFileContent(loc.Path, loc.Line, loc.Column)
		gitCommitCode.SetText(gitContent)
		
		// Update working set version
		workingContent := getWorkingSetContent(loc.Path)
		workingSetCode.SetText(workingContent)
	}

	window.ShowAndRun()
}
