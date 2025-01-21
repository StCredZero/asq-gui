package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type FileLocation struct {
	Path      string
	Line      int    // Starting line number
	Column    int
	LineCount int    // Number of lines in the matched text
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

// Custom color names for our theme
const (
	ColorNameMatchedText fyne.ThemeColorName = "matchedText"
)

// MyGreenBlackTheme implements a custom theme with green text on black background
type MyGreenBlackTheme struct{}

var _ fyne.Theme = (*MyGreenBlackTheme)(nil)

func (m *MyGreenBlackTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.Black
	case theme.ColorNameForeground:
		return color.RGBA{0, 255, 0, 255} // bright green
	case theme.ColorNameDisabled:
		return color.RGBA{0, 128, 0, 255} // darker green for disabled state
	case theme.ColorNameInputBackground:
		return color.Black // ensure MultiLineEntry widgets have black background
	case ColorNameMatchedText:
		return color.RGBA{0, 0, 255, 255} // bright blue for matched text
	case theme.ColorNameSeparator:
		return color.Gray{Y: 128} // medium grey for split container dividers
	// Return default colors for focus and selection to prevent blue background in list
	case theme.ColorNameFocus, theme.ColorNameSelection, theme.ColorNameHover, theme.ColorNamePressed:
		return theme.DefaultTheme().Color(name, variant)
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (m *MyGreenBlackTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *MyGreenBlackTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *MyGreenBlackTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func loadAsqFromStdin() []FileLocation {
	var locations []FileLocation
	scanner := bufio.NewScanner(os.Stdin)
	var currentLoc *FileLocation
	var matchedLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "//asq_match ") {
			// If we were collecting a previous match, save it
			if currentLoc != nil {
				currentLoc.LineCount = len(matchedLines)
				matchedLines = nil
			}
			// Start new match
			trimmed := strings.TrimPrefix(line, "//asq_match ")
			loc := parseFileLocation(trimmed)
			currentLoc = &loc
			locations = append(locations, loc)
		} else if currentLoc != nil {
			// Collect lines of the current match
			matchedLines = append(matchedLines, line)
		}
	}
	
	// Handle the last match if any
	if currentLoc != nil && len(matchedLines) > 0 {
		currentLoc.LineCount = len(matchedLines)
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

	// Create text grids for the bottom panes
	gitCommitCode := widget.NewTextGrid()
	gitCommitCode.ShowLineNumbers = false
	workingSetCode := widget.NewTextGrid()
	workingSetCode.ShowLineNumbers = false

	// Create scrollable containers for the code views
	gitScroll := container.NewScroll(gitCommitCode)
	workingScroll := container.NewScroll(workingSetCode)
	
	// Create split containers
	bottomSplit := container.NewHSplit(
		gitScroll,
		workingScroll,
	)
	bottomSplit.SetOffset(0.5) // Equal split

	mainSplit := container.NewVSplit(
		container.NewScroll(fileList),
		bottomSplit,
	)
	mainSplit.SetOffset(0.3) // 30% top, 70% bottom

	window.SetContent(mainSplit)
	window.Resize(fyne.NewSize(1024, 768))

	// Apply custom theme for green text on black background
	myApp.Settings().SetTheme(&MyGreenBlackTheme{})

	// Load initial file locations from a file or stdin
	if len(os.Args) > 1 {
		if os.Args[1] == "--display" {
			locations = loadAsqFromStdin()
			fileList.Refresh()
		} else {
			locations = loadFileLocations(os.Args[1])
			fileList.Refresh()
		}
	}

	// Handle list selection
	fileList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(locations) {
			return
		}
		loc := locations[id]
		
		// Update git commit version
		gitContent := getGitFileContent(loc.Path, loc.Line, loc.Column)
		lines := strings.Split(gitContent, "\n")
		gitCommitCode.Resize(fyne.NewSize(gitCommitCode.Size().Width, float32(len(lines))))
		
		// Calculate line height for scrolling (assuming monospace font)
		lineHeight := gitCommitCode.MinSize().Height / float32(len(lines))
		
		for rowIndex, lineStr := range lines {
			var row widget.TextGridRow
			for _, r := range lineStr {
				row.Cells = append(row.Cells, widget.TextGridCell{Rune: r})
			}
			gitCommitCode.SetRow(rowIndex, row)
			
			// Set default green style for all text
			greenStyle := &widget.CustomTextGridStyle{
				FGColor: color.RGBA{0, 255, 0, 255}, // bright green
				BGColor: color.Black,
			}
			gitCommitCode.SetStyleRange(rowIndex, 0, rowIndex, len(lineStr)-1, greenStyle)
			
			// Apply blue color to matched line range (convert from 1-based to 0-based index)
			if rowIndex >= loc.Line-1 && rowIndex < loc.Line-1+loc.LineCount {
				blueStyle := &widget.CustomTextGridStyle{
					FGColor: color.RGBA{0, 0, 255, 255}, // bright blue
					BGColor: color.Black,
				}
				gitCommitCode.SetStyleRange(rowIndex, 0, rowIndex, len(lineStr)-1, blueStyle)
			}
		}
		
		// Scroll to matched line range
		matchedLineY := lineHeight * float32(loc.Line-1)
		gitScroll.Offset = fyne.NewPos(0, matchedLineY)
		gitScroll.Refresh()
		
		// Update working set version
		workingContent := getWorkingSetContent(loc.Path)
		lines = strings.Split(workingContent, "\n")
		workingSetCode.Resize(fyne.NewSize(workingSetCode.Size().Width, float32(len(lines))))
		
		for rowIndex, lineStr := range lines {
			var row widget.TextGridRow
			for _, r := range lineStr {
				row.Cells = append(row.Cells, widget.TextGridCell{Rune: r})
			}
			workingSetCode.SetRow(rowIndex, row)
			
			// Set default green style for all text
			greenStyle := &widget.CustomTextGridStyle{
				FGColor: color.RGBA{0, 255, 0, 255}, // bright green
				BGColor: color.Black,
			}
			workingSetCode.SetStyleRange(rowIndex, 0, rowIndex, len(lineStr)-1, greenStyle)
			
			// Apply blue color to matched line range (convert from 1-based to 0-based index)
			if rowIndex >= loc.Line-1 && rowIndex < loc.Line-1+loc.LineCount {
				blueStyle := &widget.CustomTextGridStyle{
					FGColor: color.RGBA{0, 0, 255, 255}, // bright blue
					BGColor: color.Black,
				}
				workingSetCode.SetStyleRange(rowIndex, 0, rowIndex, len(lineStr)-1, blueStyle)
			}
		}
		
		// Scroll working set to matched line range
		workingScroll.Offset = fyne.NewPos(0, matchedLineY)
		workingScroll.Refresh()
	}

	window.ShowAndRun()
}
