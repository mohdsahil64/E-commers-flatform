package support

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"
)

// Event is what the Hub pushes to connected portals in real time.
//
//   - type "message":        a new chat message landed (Message is set).
//   - type "ticket_changed": a ticket was created / resolved; clients should
//     refetch their ticket list.
type Event struct {
	// Scope is empty for customer tickets and "seller_support" for the
	// seller -> admin tickets, so each page can ignore the other's events.
	Scope    string   `json:"scope,omitempty"`
	Type     string   `json:"type"`
	TicketID string   `json:"ticket_id"`
	Status   string   `json:"status,omitempty"`
	Message  *Message `json:"message,omitempty"`
}

// Audience says who may receive an event for a ticket.
type Audience struct {
	CustomerUserID string // users.id of the ticket's customer
	SellerID       string // sellers.id of the owning seller (product tickets)
}

// Client is one connected portal session.
type Client struct {
	ws       *wsConn
	role     string // customer | seller | admin
	userID   string
	sellerID string // seller portal only
	send     chan []byte
}

// NewClient builds a Client for the given identity.
func NewClient(role, userID, sellerID string) *Client {
	return &Client{role: role, userID: userID, sellerID: sellerID, send: make(chan []byte, 64)}
}

// Hub fans events out to the right connected clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// NewHub builds an empty Hub.
func NewHub() *Hub { return &Hub{clients: map[*Client]struct{}{}} }

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// wants reports whether a client should receive an event for the audience.
// Admins (with support.manage, verified at connect time) receive everything:
// they own non-product tickets and may audit product tickets read-only.
func (c *Client) wants(a Audience) bool {
	switch c.role {
	case "customer":
		return a.CustomerUserID != "" && c.userID == a.CustomerUserID
	case "seller":
		return a.SellerID != "" && c.sellerID == a.SellerID
	case "admin":
		return true
	}
	return false
}

// Publish delivers an event to every interested connected client.
func (h *Hub) Publish(ev Event, a Audience) {
	if h == nil {
		return
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.wants(a) {
			continue
		}
		select {
		case c.send <- data:
		default:
			// Slow consumer: drop the frame rather than block the request path.
			// The client resyncs from the REST API on reconnect.
		}
	}
}

// Serve registers the client and runs its read/write loops on a hijacked
// connection until it closes. It blocks, so call it from the Hijack handler.
func (h *Hub) Serve(conn net.Conn, c *Client) {
	defer conn.Close()
	c.ws = &wsConn{conn: conn}
	h.add(c)
	defer h.remove(c)

	done := make(chan struct{})
	defer close(done)

	// Writer: pushes events and sends keep-alive pings.
	go func() {
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-done:
				return
			case msg := <-c.send:
				if err := c.ws.writeFrame(opText, msg); err != nil {
					_ = conn.Close()
					return
				}
			case <-ping.C:
				if err := c.ws.writeFrame(opPing, nil); err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()

	// Reader: handles control frames and detects disconnects. Any frame (the
	// browser auto-answers our pings with pongs) extends the read deadline.
	r := bufio.NewReader(conn)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
		op, payload, err := readFrame(r)
		if err != nil {
			return
		}
		switch op {
		case opClose:
			_ = c.ws.writeFrame(opClose, nil)
			return
		case opPing:
			_ = c.ws.writeFrame(opPong, payload)
		}
	}
}
