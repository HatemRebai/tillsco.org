package tracks

import (
	"sync"

	"github.com/jeremija/peer-calls/src/server-go/logger"
	"github.com/pion/webrtc/v2"
)

var log = logger.GetLogger("tracks")

type TracksManager struct {
	mu sync.RWMutex
	// key is clientID
	peers map[string]peerInRoom
	// key is room, value is clientID
	peerIDsByRoom map[string]map[string]struct{}
}

func NewTracksManager() *TracksManager {
	return &TracksManager{
		peers:         map[string]peerInRoom{},
		peerIDsByRoom: map[string]map[string]struct{}{},
	}
}

type peerInRoom struct {
	peer *Peer
	room string
}

func (t *TracksManager) addTrack(room string, clientID string, track *webrtc.Track) {
	t.mu.Lock()

	// for otherClientID, otherPeerInRoom := range t.peers {
	for _, otherPeerInRoom := range t.peers {
		// if otherClientID != clientID {
		otherPeerInRoom.peer.AddTrack(track)
		// }
	}

	t.mu.Unlock()
}

func (t *TracksManager) Add(room string, clientID string, peerConnection PeerConnection, onTrack func(kind string)) {
	log.Printf("Add peer to TrackManager room: %s, clientID: %s", room, clientID)

	peer := NewPeer(
		clientID,
		peerConnection,
		func(clientID string, track *webrtc.Track) {
			t.addTrack(room, clientID, track)
			onTrack(track.Kind().String())
		},
		t.removePeer,
	)

	t.mu.Lock()
	peerJoiningRoom := peerInRoom{
		peer: peer,
		room: room,
	}

	peersSet, ok := t.peerIDsByRoom[room]
	if !ok {
		peersSet = map[string]struct{}{}
		t.peerIDsByRoom[room] = peersSet
	}

	for existingPeerClientID := range peersSet {
		existingPeerInRoom, ok := t.peers[existingPeerClientID]
		if !ok {
			log.Printf("Cannot find existing peer with clientID: %s", existingPeerClientID)
			continue
		}
		for _, track := range existingPeerInRoom.peer.Tracks() {
			err := peerJoiningRoom.peer.AddTrack(track)
			if err != nil {
				log.Printf(
					"Error adding peer clientID: %s track to clientID: %s - reason: %s",
					existingPeerClientID,
					clientID,
					err,
				)
				continue
			}
		}
	}

	t.peers[clientID] = peerJoiningRoom
	peersSet[clientID] = struct{}{}
	t.mu.Unlock()
}

func (t *TracksManager) removePeer(clientID string) {
	log.Printf("removePeer: %s", clientID)
	t.mu.Lock()
	defer t.mu.Unlock()
	peerLeavingRoom, ok := t.peers[clientID]
	if !ok {
		log.Printf("Cannot remove peer clientID: %s (not found)", clientID)
		return
	}

	t.removePeerTracks(peerLeavingRoom)

	delete(t.peers, clientID)
	peerIDs, ok := t.peerIDsByRoom[peerLeavingRoom.room]
	if !ok {
		log.Printf("Cannot remove peer ID from room: %s (not found)", clientID)
		return
	}
	delete(peerIDs, clientID)
}

func (t *TracksManager) removePeerTracks(peerLeavingRoom peerInRoom) {
	leavingClientID := peerLeavingRoom.peer.ClientID()
	log.Printf("Remove all peer tracks for clientID: %s", leavingClientID)
	clientIDs, ok := t.peerIDsByRoom[peerLeavingRoom.room]
	if !ok {
		log.Println("Cannot find any peers in room", peerLeavingRoom.room)
		return
	}

	tracks := peerLeavingRoom.peer.Tracks()
	for clientID := range clientIDs {
		if clientID != leavingClientID {
			otherPeerInRoom := t.peers[clientID]
			for _, track := range tracks {
				log.Printf(
					"Removing track: %s from peer clientID: %s (source clientID: %s)",
					track.ID(),
					clientID,
					leavingClientID,
				)
				err := otherPeerInRoom.peer.RemoveTrack(track)
				if err != nil {
					log.Printf(
						"Error removing track: %s from peer clientID: %s (source clientID: %s): %s",
						track.ID(),
						clientID,
						leavingClientID,
						err,
					)
				}
			}
		}
	}
}