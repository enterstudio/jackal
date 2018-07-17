/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package stream

import (
	"errors"
	"time"

	"github.com/ortuman/jackal/xmpp"
	"github.com/ortuman/jackal/xmpp/jid"
)

// InStream represents a generic incoming stream.
type InStream interface {
	ID() string
	Disconnect(err error)
}

// InOutStream represents a generic incoming/outgoing stream.
type InOutStream interface {
	InStream
	SendElement(elem xmpp.XElement)
}

// C2S represents a client-to-server XMPP stream.
type C2S interface {
	InOutStream

	Context() Context

	Username() string
	Domain() string
	Resource() string

	JID() *jid.JID

	IsSecured() bool
	IsAuthenticated() bool
	IsCompressed() bool

	Presence() *xmpp.Presence
}

// S2SIn represents an incoming server-to-server XMPP stream.
type S2SIn interface {
	InStream
}

// S2SOut represents an outgoing server-to-server XMPP stream.
type S2SOut interface {
	InOutStream
}

// MockC2S represents a mocked c2s stream.
type MockC2S struct {
	id      string
	ctx     Context
	elemCh  chan xmpp.XElement
	actorCh chan func()
	discCh  chan error
	doneCh  chan<- struct{}
}

// NewMockC2S returns a new mocked stream instance.
func NewMockC2S(id string, jid *jid.JID) *MockC2S {
	ctx, doneCh := NewContext()
	stm := &MockC2S{
		id:      id,
		ctx:     ctx,
		elemCh:  make(chan xmpp.XElement, 16),
		actorCh: make(chan func(), 64),
		discCh:  make(chan error, 1),
		doneCh:  doneCh,
	}
	stm.ctx.SetObject(jid, "jid")
	stm.ctx.SetString(jid.Node(), "username")
	stm.ctx.SetString(jid.Domain(), "domain")
	stm.ctx.SetString(jid.Resource(), "resource")
	go stm.actorLoop()
	return stm
}

// ID returns mocked stream identifier.
func (m *MockC2S) ID() string {
	return m.id
}

// Context returns mocked stream associated context.
func (m *MockC2S) Context() Context {
	return m.ctx
}

// Username returns current mocked stream username.
func (m *MockC2S) Username() string {
	return m.ctx.String("username")
}

// SetUsername sets the mocked stream username value.
func (m *MockC2S) SetUsername(username string) {
	m.ctx.SetString(username, "username")
}

// Domain returns current mocked stream domain.
func (m *MockC2S) Domain() string {
	return m.ctx.String("domain")
}

// SetDomain sets the mocked stream domain value.
func (m *MockC2S) SetDomain(domain string) {
	m.ctx.SetString(domain, "domain")
}

// Resource returns current mocked stream resource.
func (m *MockC2S) Resource() string {
	return m.ctx.String("resource")
}

// SetResource sets the mocked stream resource value.
func (m *MockC2S) SetResource(resource string) {
	m.ctx.SetString(resource, "resource")
}

// JID returns current user JID.
func (m *MockC2S) JID() *jid.JID {
	return m.ctx.Object("jid").(*jid.JID)
}

// SetJID sets the mocked stream JID value.
func (m *MockC2S) SetJID(jid *jid.JID) {
	m.ctx.SetObject(jid, "jid")
}

// SetSecured sets whether or not the a mocked stream
// has been secured.
func (m *MockC2S) SetSecured(secured bool) {
	m.ctx.SetBool(secured, "secured")
}

// IsSecured returns whether or not the mocked stream
// has been secured.
func (m *MockC2S) IsSecured() bool {
	return m.ctx.Bool("secured")
}

// SetAuthenticated sets whether or not the a mocked stream
// has been authenticated.
func (m *MockC2S) SetAuthenticated(authenticated bool) {
	m.ctx.SetBool(authenticated, "authenticated")
}

// IsAuthenticated returns whether or not the mocked stream
// has successfully authenticated.
func (m *MockC2S) IsAuthenticated() bool {
	return m.ctx.Bool("authenticated")
}

// SetCompressed sets whether or not the a mocked stream
// has been compressed.
func (m *MockC2S) SetCompressed(compressed bool) {
	m.ctx.SetBool(compressed, "compressed")
}

// IsCompressed returns whether or not the mocked stream
// has enabled a compression method.
func (m *MockC2S) IsCompressed() bool {
	return m.ctx.Bool("compressed")
}

// IsDisconnected returns whether or not the mocked stream has been disconnected.
func (m *MockC2S) IsDisconnected() bool {
	return m.ctx.Bool("disconnected")
}

// SetPresence sets the mocked stream last received
// presence element.
func (m *MockC2S) SetPresence(presence *xmpp.Presence) {
	m.ctx.SetObject(presence, "presence")
}

// Presence returns last sent presence element.
func (m *MockC2S) Presence() *xmpp.Presence {
	switch v := m.ctx.Object("presence").(type) {
	case *xmpp.Presence:
		return v
	}
	return nil
}

// SendElement sends the given XML element.
func (m *MockC2S) SendElement(elem xmpp.XElement) {
	m.actorCh <- func() {
		m.sendElement(elem)
	}
}

// Disconnect disconnects mocked stream.
func (m *MockC2S) Disconnect(err error) {
	waitCh := make(chan struct{})
	m.actorCh <- func() {
		m.disconnect(err)
		close(waitCh)
	}
	<-waitCh
}

// FetchElement waits until a new XML element is sent to
// the mocked stream and returns it.
func (m *MockC2S) FetchElement() xmpp.XElement {
	select {
	case e := <-m.elemCh:
		return e
	case <-time.After(time.Second * 5):
		return &xmpp.Element{}
	}
}

// WaitDisconnection waits until the mocked stream disconnects.
func (m *MockC2S) WaitDisconnection() error {
	select {
	case err := <-m.discCh:
		return err
	case <-time.After(time.Second * 5):
		return errors.New("operation timed out")
	}
}

func (m *MockC2S) actorLoop() {
	for {
		select {
		case f := <-m.actorCh:
			f()
		case <-m.discCh:
			return
		}
	}
}

func (m *MockC2S) sendElement(elem xmpp.XElement) {
	select {
	case m.elemCh <- elem:
		return
	default:
		break
	}
}

func (m *MockC2S) disconnect(err error) {
	if !m.ctx.Bool("disconnected") {
		m.discCh <- err
		close(m.doneCh)
		m.ctx.SetBool(true, "disconnected")
	}
}
