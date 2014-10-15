package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Menu struct {
	TargetID         int
	CommandHistory   []*Command
	ResponseHistory  []*CommandResponse
	WaitingResponses []*Command
	CommandQueue     []*Command
	Conn             net.Conn
	Ready            bool
	Displayed        bool
	Parent           *Menu
	Children         []*Menu
}

func (menu *Menu) Create(conn net.Conn) {
	menuCreate := Command{
		Action:     "create",
		ObjectType: "menu",
	}
	menu.Send(&menuCreate, conn)
}

func (menu *Menu) IsTarget(targetId int) bool {
	return targetId == menu.TargetID
}
func (menu *Menu) HandleError(reply CommandResponse) {

}

func (menu *Menu) HandleEvent(reply CommandResponse) {

}

func (menu *Menu) HandleReply(reply CommandResponse, conn net.Conn) {
	fmt.Println("MENU::Handling Response", reply)
	for k, v := range menu.WaitingResponses {
		if v.ID != reply.ID {
			continue
		}
		if len(menu.WaitingResponses) > 1 {
			menu.WaitingResponses = menu.WaitingResponses[:k+copy(menu.WaitingResponses[k:], menu.WaitingResponses[k+1:])]
		} else {
			menu.WaitingResponses = []*Command{}
		}

		if menu.TargetID == 0 && v.Action == "create" {
			//Assume we have a reply to action:create
			if reply.Result.TargetID != 0 {
				menu.TargetID = reply.Result.TargetID
				fmt.Println("Received TargetID", "\nSetting Ready State")
				menu.Ready = true
			}
			for i, _ := range menu.CommandQueue {
				menu.CommandQueue[i].TargetID = menu.TargetID
				menu.Send(menu.CommandQueue[i], conn)
			}
			// Reinitialize empty command queue, and allow gc.
			menu.CommandQueue = []*Command{}
			return
		}

		if v.Action == "call" && v.Method == "set_application_menu" {
			menu.Displayed = true
		}

	}
}

func (menu *Menu) Send(command *Command, conn net.Conn) {
	ActionId += 1

	command.ID = ActionId

	fmt.Println(command)
	cmd, _ := json.Marshal(&command)
	fmt.Println("Writing", string(cmd), "\n", SOCKET_BOUNDARY)

	menu.WaitingResponses = append(menu.WaitingResponses, command)

	conn.Write(cmd)
	conn.Write([]byte("\n"))
	conn.Write([]byte(SOCKET_BOUNDARY))
}

func (menu *Menu) Call(command *Command, conn net.Conn) {
	command.Action = "call"
	command.TargetID = menu.TargetID
	if menu.Ready == false {
		menu.CommandQueue = append(menu.CommandQueue, command)
		return
	}
	menu.Send(command, conn)
}

func (menu *Menu) AddItem(commandID int, label string, conn net.Conn) {
	command := Command{
		Method: "add_item",
		Args: CommandArguments{
			CommandID: commandID,
			Label:     label,
		},
	}

	menu.Call(&command, conn)
}

func (menu *Menu) AddSubmenu(commandID int, label string, child *Menu, conn net.Conn) {
	command := Command{
		Method: "add_submenu",
		Args: CommandArguments{
			CommandID: commandID,
			Label:     label,
			MenuID:    child.TargetID,
		},
	}

	// Assign Bidirectional navigation elements i.e. DoublyLinkedLists
	child.Parent = menu
	menu.Children = append(menu.Children, child)

	go func() {
		for {
			if child.IsStable() {
				menu.Call(&command, conn)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
}

func (menu *Menu) AddSeperator(conn net.Conn) {
	command := Command{
		Method: "add_seperator",
	}

	menu.Call(&command, conn)
}

func (menu *Menu) SetApplicationMenu(conn net.Conn) {
	command := Command{
		Method: "set_application_menu",
		Args: CommandArguments{
			MenuID: menu.TargetID,
		},
	}
	menu.Displayed = true
	// Thread to wait for Stable Menu State
	go func() {
		for {
			if menu.IsMenuTreeReady() {
				menu.Call(&command, conn)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

}

func (menu *Menu) IsStable() bool {
	return menu.Ready && len(menu.WaitingResponses) == 0
}

func (menu *Menu) IsMenuTreeReady() bool {
	if !menu.IsStable() {
		return false
	}

	for _, child := range menu.Children {
		//fmt.Println("Checking child")
		if !child.IsMenuTreeReady() {
			return false
		}
	}

	return true
}
