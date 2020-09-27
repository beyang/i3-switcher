package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const (
	levelSize = 100
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("no args")
	}

	directive := os.Args[1]
	var adjuster string
	if len(os.Args) >= 3 {
		adjuster = os.Args[2]
	}
	switch directive {
	case "right":
		return move(1, adjuster == "container")
	case "left":
		return move(-1, adjuster == "container")
	case "insert":
		return insert(adjuster == "container")
	case "endsert":
		return endsert(adjuster == "container")
	case "newLevel":
		return insertLevel(adjuster == "container")
	case "down":
		return moveLevelDown(adjuster == "container")
	case "up":
		return moveLevelUp(adjuster == "container")
	default:
		return fmt.Errorf("Unrecognized directive %q\n", directive)
	}
	return nil
}

func getFocused(workspaces []*i3Workspace) (*i3Workspace, int) {
	for i, w := range workspaces {
		if w.Focused {
			return w, i
		}
	}
	return nil, -1
}

func insertLevel(moveContainer bool) error {
	maxLevels := 1000
	workspaces, err := getWorkspaces()
	if err != nil {
		return err
	}
	_, i_focused := getFocused(workspaces)
	if i_focused < 0 {
		return nil
	}
	for level := workspaces[i_focused].Num/levelSize + 1; level < maxLevels; level++ {
		count := 0
		for _, w := range workspaces {
			if w.Num/levelSize == level {
				count++
			}
		}
		if count == 0 {
			recordLevelSwitch(workspaces[i_focused].Num)
			return switchToWorkspace((level*levelSize + 1), moveContainer)
		}
	}
	return fmt.Errorf("Could not create new level after looking through %d levels", maxLevels)
}

func insert(moveContainer bool) error {
	workspaces, err := getWorkspaces()
	if err != nil {
		return err
	}
	_, i_focused := getFocused(workspaces)
	if i_focused < 0 {
		return nil
	}
	var toShift []*i3Workspace
	for i := i_focused + 1; i < len(workspaces); i++ {
		if workspaces[i].Num > workspaces[i-1].Num+1 {
			break
		}
		if workspaces[i].Num/levelSize != workspaces[i_focused].Num/levelSize {
			break
		}
		toShift = append(toShift, workspaces[i])
	}
	if err := shift(toShift); err != nil {
		return err
	}
	return switchToWorkspace(workspaces[i_focused].Num+1, moveContainer)
}

func endsert(moveContainer bool) error {
	workspaces, err := getWorkspaces()
	if err != nil {
		return err
	}
	_, i_focused := getFocused(workspaces)
	if i_focused < 0 {
		return nil
	}
	for i := i_focused + 1; i < len(workspaces); i++ {
		if workspaces[i].Num/levelSize != workspaces[i_focused].Num/levelSize {
			return switchToWorkspace(workspaces[i-1].Num+1, moveContainer)
		}
	}
	return switchToWorkspace(workspaces[len(workspaces)-1].Num+1, moveContainer)
}

func shift(workspaces []*i3Workspace) error {
	for i := len(workspaces) - 1; i >= 0; i-- {
		num := workspaces[i].Num
		if err := exec.Command("i3-msg", "rename", "workspace", strconv.Itoa(num), "to", strconv.Itoa(num+1)).Run(); err != nil {
			return err
		}
	}
	return nil
}

func moveLevelDown(moveContainer bool) error {
	workspaces, err := getWorkspaces()
	if err != nil {
		return err
	}
	_, i_focused := getFocused(workspaces)
	if i_focused < 0 {
		return nil
	}
	for i := i_focused + 1; i < len(workspaces); i++ {
		if workspaces[i].Num/levelSize > workspaces[i_focused].Num/levelSize {
			recordLevelSwitch(workspaces[i_focused].Num)
			return moveToLevel(workspaces, workspaces[i].Num/levelSize, moveContainer)
		}
	}
	return nil
}

func moveLevelUp(moveContainer bool) error {
	workspaces, err := getWorkspaces()
	if err != nil {
		return err
	}
	_, i_focused := getFocused(workspaces)
	if i_focused < 0 {
		return nil
	}
	for i := i_focused - 1; i >= 0; i-- {
		if workspaces[i].Num/levelSize < workspaces[i_focused].Num/levelSize {
			recordLevelSwitch(workspaces[i_focused].Num)
			return moveToLevel(workspaces, workspaces[i].Num/levelSize, moveContainer)
		}
	}
	return nil
}

func move(m int, moveContainer bool) error {
	workspaces, err := getWorkspaces()
	if err != nil {
		return err
	}
	_, i_focused := getFocused(workspaces)
	if i_focused < 0 {
		return nil
	}

	i_next := i_focused + m
	if i_next < 0 || i_next >= len(workspaces) {
		return nil
	}

	curr, next := workspaces[i_focused], workspaces[i_next]
	if next.Num/levelSize != curr.Num/levelSize {
		return nil
	}
	return switchToWorkspace(next.Num, moveContainer)
}

type i3Workspace struct {
	ID      int64  `json:"id"`
	Num     int    `json:"num"`
	Name    string `json:"name"`
	Visible bool   `json:"visible"`
	Focused bool   `json:"focused"`
	Output  string `json:"output"`
	Urgent  bool   `json:"urgent"`
}

func getWorkspaces() (workspaces []*i3Workspace, err error) {
	b, err := exec.Command("i3-msg", "-t", "get_workspaces").Output()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &workspaces); err != nil {
		return nil, err
	}
	return workspaces, nil
}

func switchToWorkspace(n int, moveContainer bool) error {
	if moveContainer {
		logErr(exec.Command("i3-msg", "move", "container", "to", "workspace", strconv.Itoa(n)).Run())
	}
	return exec.Command("i3-msg", "workspace", strconv.Itoa(n)).Run()
}

func logErr(err error) {
	if err == nil {
		return
	}
	fmt.Println(err.Error())
}

func getStateFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".i3-switcher", "state.json"), nil
}

func moveToLevel(workspaces []*i3Workspace, level int, moveContainer bool) error {
	if s := readState(); s != nil {
		if lastVisited, ok := s.LastWorkspaceVisitedByLevel[level]; ok {
			return switchToWorkspace(lastVisited, moveContainer)
		}
	}
	for _, workspace := range workspaces {
		if workspace.Num/levelSize == level {
			return switchToWorkspace(workspace.Num, moveContainer)
		}
	}
	return nil
}

func readState() *state {
	stateFile, err := getStateFile()
	if err != nil {
		logErr(err)
		return nil
	}
	b, err := ioutil.ReadFile(stateFile)
	if err != nil {
		logErr(err)
		return nil
	}
	var s state
	if err := json.Unmarshal(b, &s); err != nil {
		logErr(err)
		return nil
	}
	return &s
}

func writeState(s *state) error {
	stateFile, err := getStateFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(stateFile), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(stateFile, b, 0644)
}

func recordLevelSwitch(currentWorkspaceNum int) {
	currentLevel := currentWorkspaceNum / levelSize
	s := readState()
	if s == nil {
		s = &state{}
	}
	if s.LastWorkspaceVisitedByLevel == nil {
		s.LastWorkspaceVisitedByLevel = map[int]int{}
	}
	s.LastWorkspaceVisitedByLevel[currentLevel] = currentWorkspaceNum
	logErr(writeState(s))
}

type state struct {
	// Tracks the last visited workspace number by level
	LastWorkspaceVisitedByLevel map[int]int
}
