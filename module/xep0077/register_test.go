/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package xep0077

import (
	"testing"

	"github.com/ortuman/jackal/model"
	"github.com/ortuman/jackal/storage"
	"github.com/ortuman/jackal/stream"
	"github.com/ortuman/jackal/xmpp"
	"github.com/ortuman/jackal/xmpp/jid"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

func TestXEP0077_Matching(t *testing.T) {
	j, _ := jid.New("ortuman", "jackal.im", "balcony", true)

	x := New(&Config{}, nil)

	// test MatchesIQ
	iq := xmpp.NewIQType(uuid.New(), xmpp.SetType)
	iq.SetFromJID(j)

	require.False(t, x.MatchesIQ(iq))
	iq.AppendElement(xmpp.NewElementNamespace("query", registerNamespace))
	require.True(t, x.MatchesIQ(iq))
}

func TestXEP0077_InvalidToJID(t *testing.T) {
	j, _ := jid.New("ortuman", "jackal.im", "balcony", true)

	stm := stream.NewMockC2S("abcd1234", j)
	defer stm.Disconnect(nil)

	x := New(&Config{}, stm)

	stm.SetUsername("romeo")
	iq := xmpp.NewIQType(uuid.New(), xmpp.SetType)
	iq.SetFromJID(j)
	iq.SetToJID(j.ToBareJID())

	x.ProcessIQ(iq)
	elem := stm.FetchElement()
	require.Equal(t, xmpp.ErrForbidden.Error(), elem.Error().Elements().All()[0].Name())

	iq2 := xmpp.NewIQType(uuid.New(), xmpp.SetType)
	iq2.SetFromJID(j)
	iq2.SetToJID(j.ToBareJID())

	stm.SetUsername("ortuman")
	stm.SetAuthenticated(true)
	x.ProcessIQ(iq2)
	elem = stm.FetchElement()
	require.Equal(t, "iq", elem.Name())
	require.Equal(t, xmpp.ErrForbidden.Error(), elem.Error().Elements().All()[0].Name())
}

func TestXEP0077_NotAuthenticatedErrors(t *testing.T) {
	j, _ := jid.New("ortuman", "jackal.im", "balcony", true)

	stm := stream.NewMockC2S("abcd1234", j)
	defer stm.Disconnect(nil)

	x := New(&Config{}, stm)

	iq := xmpp.NewIQType(uuid.New(), xmpp.ResultType)
	iq.SetFromJID(j)
	iq.SetToJID(j.ToBareJID())

	x.ProcessIQ(iq)
	elem := stm.FetchElement()
	require.Equal(t, xmpp.ErrBadRequest.Error(), elem.Error().Elements().All()[0].Name())

	iq.SetType(xmpp.GetType)
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrNotAllowed.Error(), elem.Error().Elements().All()[0].Name())

	// allow registration...
	x = New(&Config{AllowRegistration: true}, stm)

	q := xmpp.NewElementNamespace("query", registerNamespace)
	q.AppendElement(xmpp.NewElementName("q2"))
	iq.AppendElement(q)

	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrBadRequest.Error(), elem.Error().Elements().All()[0].Name())

	q.ClearElements()
	iq.SetType(xmpp.SetType)
	x.registered = true

	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrNotAcceptable.Error(), elem.Error().Elements().All()[0].Name())
}

func TestXEP0077_AuthenticatedErrors(t *testing.T) {
	srvJid, _ := jid.New("", "jackal.im", "", true)
	j, _ := jid.New("ortuman", "jackal.im", "balcony", true)

	stm := stream.NewMockC2S("abcd1234", j)
	defer stm.Disconnect(nil)

	stm.SetAuthenticated(true)

	x := New(&Config{}, stm)

	iq := xmpp.NewIQType(uuid.New(), xmpp.ResultType)
	iq.SetFromJID(j)
	iq.SetToJID(j.ToBareJID())
	iq.SetToJID(srvJid)

	x.ProcessIQ(iq)
	elem := stm.FetchElement()
	require.Equal(t, xmpp.ErrBadRequest.Error(), elem.Error().Elements().All()[0].Name())

	iq.SetType(xmpp.SetType)
	iq.AppendElement(xmpp.NewElementNamespace("query", registerNamespace))
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrBadRequest.Error(), elem.Error().Elements().All()[0].Name())
}

func TestXEP0077_RegisterUser(t *testing.T) {
	storage.Initialize(&storage.Config{Type: storage.Memory})
	defer storage.Shutdown()

	srvJid, _ := jid.New("", "jackal.im", "", true)
	j, _ := jid.New("ortuman", "jackal.im", "balcony", true)

	stm := stream.NewMockC2S("abcd1234", j)
	defer stm.Disconnect(nil)

	x := New(&Config{AllowRegistration: true}, stm)

	iq := xmpp.NewIQType(uuid.New(), xmpp.GetType)
	iq.SetFromJID(srvJid)
	iq.SetToJID(srvJid)

	q := xmpp.NewElementNamespace("query", registerNamespace)
	iq.AppendElement(q)

	x.ProcessIQ(iq)
	q2 := stm.FetchElement().Elements().ChildNamespace("query", registerNamespace)
	require.NotNil(t, q2.Elements().Child("username"))
	require.NotNil(t, q2.Elements().Child("password"))

	username := xmpp.NewElementName("username")
	password := xmpp.NewElementName("password")
	q.AppendElement(username)
	q.AppendElement(password)

	// empty fields
	iq.SetType(xmpp.SetType)
	x.ProcessIQ(iq)
	elem := stm.FetchElement()
	require.Equal(t, xmpp.ErrBadRequest.Error(), elem.Error().Elements().All()[0].Name())

	// already existing user...
	storage.Instance().InsertOrUpdateUser(&model.User{Username: "ortuman", Password: "1234"})
	username.SetText("ortuman")
	password.SetText("5678")
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrConflict.Error(), elem.Error().Elements().All()[0].Name())

	// storage error
	storage.ActivateMockedError()
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrInternalServerError.Error(), elem.Error().Elements().All()[0].Name())

	storage.DeactivateMockedError()
	username.SetText("juliet")
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ResultType, elem.Type())

	usr, _ := storage.Instance().FetchUser("ortuman")
	require.NotNil(t, usr)
}

func TestXEP0077_CancelRegistration(t *testing.T) {
	storage.Initialize(&storage.Config{Type: storage.Memory})
	defer storage.Shutdown()

	srvJid, _ := jid.New("", "jackal.im", "", true)
	j, _ := jid.New("ortuman", "jackal.im", "balcony", true)

	stm := stream.NewMockC2S("abcd1234", j)
	defer stm.Disconnect(nil)

	stm.SetAuthenticated(true)

	x := New(&Config{}, stm)

	storage.Instance().InsertOrUpdateUser(&model.User{Username: "ortuman", Password: "1234"})

	iq := xmpp.NewIQType(uuid.New(), xmpp.SetType)
	iq.SetFromJID(srvJid)
	iq.SetToJID(srvJid)

	q := xmpp.NewElementNamespace("query", registerNamespace)
	q.AppendElement(xmpp.NewElementName("remove"))

	iq.AppendElement(q)
	x.ProcessIQ(iq)
	elem := stm.FetchElement()
	require.Equal(t, xmpp.ErrNotAllowed.Error(), elem.Error().Elements().All()[0].Name())

	x = New(&Config{AllowCancel: true}, stm)

	q.AppendElement(xmpp.NewElementName("remove2"))
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrBadRequest.Error(), elem.Error().Elements().All()[0].Name())
	q.ClearElements()
	q.AppendElement(xmpp.NewElementName("remove"))

	// storage error
	storage.ActivateMockedError()
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrInternalServerError.Error(), elem.Error().Elements().All()[0].Name())
	storage.DeactivateMockedError()

	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ResultType, elem.Type())

	usr, _ := storage.Instance().FetchUser("ortuman")
	require.Nil(t, usr)
}

func TestXEP0077_ChangePassword(t *testing.T) {
	storage.Initialize(&storage.Config{Type: storage.Memory})
	defer storage.Shutdown()

	srvJid, _ := jid.New("", "jackal.im", "", true)
	j, _ := jid.New("ortuman", "jackal.im", "balcony", true)

	stm := stream.NewMockC2S("abcd1234", j)
	defer stm.Disconnect(nil)

	stm.SetAuthenticated(true)

	x := New(&Config{}, stm)

	storage.Instance().InsertOrUpdateUser(&model.User{Username: "ortuman", Password: "1234"})

	iq := xmpp.NewIQType(uuid.New(), xmpp.SetType)
	iq.SetFromJID(srvJid)
	iq.SetToJID(srvJid)

	q := xmpp.NewElementNamespace("query", registerNamespace)
	username := xmpp.NewElementName("username")
	username.SetText("juliet")
	password := xmpp.NewElementName("password")
	password.SetText("5678")
	q.AppendElement(username)
	q.AppendElement(password)
	iq.AppendElement(q)

	x.ProcessIQ(iq)
	elem := stm.FetchElement()
	require.Equal(t, xmpp.ErrNotAllowed.Error(), elem.Error().Elements().All()[0].Name())

	x = New(&Config{AllowChange: true}, stm)

	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrNotAllowed.Error(), elem.Error().Elements().All()[0].Name())

	username.SetText("ortuman")
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrNotAuthorized.Error(), elem.Error().Elements().All()[0].Name())

	// secure channel...
	stm.SetSecured(true)

	// storage error
	storage.ActivateMockedError()
	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ErrInternalServerError.Error(), elem.Error().Elements().All()[0].Name())
	storage.DeactivateMockedError()

	x.ProcessIQ(iq)
	elem = stm.FetchElement()
	require.Equal(t, xmpp.ResultType, elem.Type())

	usr, _ := storage.Instance().FetchUser("ortuman")
	require.NotNil(t, usr)
	require.Equal(t, "5678", usr.Password)
}
