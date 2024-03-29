package red

import (
	"log"
	"sync"

	"github.com/zayaanra/RED/api"
	"github.com/zayaanra/RED/internal/crdt"
	"github.com/zayaanra/RED/internal/handler"
)

type RServer struct {
	// The address for this server
	addr string

	// The handler for this server
	handler *handler.Handler

	// List of peers that are connected to this REDServer's editing session
	peers []string

	// Used to send the GUI necessary updates to catch up with it's peers editing session
	updates chan string

	crdt *crdt.CRDT

	// Denotes if this REDServer has been terminated
	terminated bool

	mutex sync.Mutex
}

// Create a new RED server associated with the given address.
// The newly created RED server begins send or receive messages immedaiately.
// This function returns an error if the server was not able to be created.
func NewREDServer(addr string, updates chan string) (api.REDServer, error) {
	rh, err := handler.NewHandler(addr)
	if err != nil {
		return nil, err
	}

	crdt := crdt.NewCRDT()
	peers := []string{}
	rs := &RServer{}
	rs.addr = addr
	rs.handler = rh
	rs.peers = peers
	rs.updates = updates
	rs.crdt = crdt
	rs.terminated = false

	go func(rh *handler.Handler) {
		for {
			select {
			case rmsg := <-rh.M:
				if rmsg == nil {
					log.Println("Killing server...")
					return
				}
				switch rmsg.Type {
				// We received an INVITE. We must add the sender of this message to our list of known peers.
				case api.MessageType_INVITE:
					log.Printf("%s accepted an INVITE from %s\n", rs.addr, rmsg.Sender)
					rs.peers = append(rs.peers, rmsg.Sender)
				// Receiveing an EDIT means that we must send the GUI the text update needed for the edtiro.
				case api.MessageType_EDIT:
					log.Printf("%s accepted an EDIT from %s\n", rs.addr, rmsg.Sender)
					text := crdt.UpdateCRDT(rmsg)
					updates <- text
				}
			}
		}
	}(rh)
	return rs, nil
}

// Invites a peer to their editing session by sending an INVITE message.
func (rs *RServer) Invite(addr string) error {
	smsg := &api.REDMessage{Type: api.MessageType_INVITE, Sender: rs.addr, Receipient: addr}
	err := rs.handler.Send(smsg, addr)
	rs.peers = append(rs.peers, addr)

	return err
}

// Notifies all peers in this editing session of an EDIT.
func (rs *RServer) Notify(char byte, pos int, editType int) {
	e := &api.Edit{Type: api.EditType(editType), Pos: int32(pos), Char: int32(char)}
	smsg := &api.REDMessage{Type: api.MessageType_EDIT, Sender: rs.addr, Edit: e}
	for _, peer := range rs.peers {
		smsg.Receipient = peer
		rs.handler.Send(smsg, peer)
	}
}

// Fetches the channel on which text updates are placed on.
func (rs *RServer) Fetch() (updates chan string) {
	return rs.updates
}

// Terminates the REDServer. It closes any resources that are currently being used.
func (rs *RServer) Terminate() {
	log.Println("Terminating server...")
	rs.terminated = true
	close(rs.updates)
	rs.handler.Terminate()
}
