/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package xep0012

import (
	"strconv"
	"time"

	"github.com/ortuman/jackal/log"
	"github.com/ortuman/jackal/model/rostermodel"
	"github.com/ortuman/jackal/module/xep0030"
	"github.com/ortuman/jackal/router"
	"github.com/ortuman/jackal/storage"
	"github.com/ortuman/jackal/stream"
	"github.com/ortuman/jackal/xmpp"
	"github.com/ortuman/jackal/xmpp/jid"
)

const lastActivityNamespace = "jabber:iq:last"

// LastActivity represents a last activity stream module.
type LastActivity struct {
	stm       stream.C2S
	startTime time.Time
}

// New returns a last activity IQ handler module.
func New(stm stream.C2S) *LastActivity {
	return &LastActivity{stm: stm, startTime: time.Now()}
}

// RegisterDisco registers disco entity features/items
// associated to last activity module.
func (x *LastActivity) RegisterDisco(discoInfo *xep0030.DiscoInfo) {
	discoInfo.Entity(x.stm.Domain(), "").AddFeature(lastActivityNamespace)
	discoInfo.Entity(x.stm.JID().ToBareJID().String(), "").AddFeature(lastActivityNamespace)
}

// MatchesIQ returns whether or not an IQ should be
// processed by the last activity module.
func (x *LastActivity) MatchesIQ(iq *xmpp.IQ) bool {
	return iq.IsGet() && iq.Elements().ChildNamespace("query", lastActivityNamespace) != nil
}

// ProcessIQ processes a last activity IQ taking according actions
// over the associated stream.
func (x *LastActivity) ProcessIQ(iq *xmpp.IQ) {
	toJID := iq.ToJID()
	if toJID.IsServer() {
		x.sendServerUptime(iq)
	} else if toJID.IsBare() {
		ri, err := storage.Instance().FetchRosterItem(x.stm.Username(), toJID.ToBareJID().String())
		if err != nil {
			log.Error(err)
			x.stm.SendElement(iq.InternalServerError())
			return
		}
		if ri != nil {
			switch ri.Subscription {
			case rostermodel.SubscriptionTo, rostermodel.SubscriptionBoth:
				x.sendUserLastActivity(iq, toJID)
				return
			}
		}
		x.stm.SendElement(iq.ForbiddenError())
	}
}

func (x *LastActivity) sendServerUptime(iq *xmpp.IQ) {
	secs := int(time.Duration(time.Now().UnixNano()-x.startTime.UnixNano()) / time.Second)
	x.sendReply(iq, secs, "")
}

func (x *LastActivity) sendUserLastActivity(iq *xmpp.IQ, to *jid.JID) {
	if len(router.UserStreams(to.Node())) > 0 { // user online
		x.sendReply(iq, 0, "")
		return
	}
	usr, err := storage.Instance().FetchUser(to.Node())
	if err != nil {
		log.Error(err)
		x.stm.SendElement(iq.InternalServerError())
		return
	}
	if usr == nil {
		x.stm.SendElement(iq.ItemNotFoundError())
		return
	}
	var secs int
	var status string
	if p := usr.LastPresence; p != nil {
		secs = int(time.Duration(time.Now().UnixNano()-usr.LastPresenceAt.UnixNano()) / time.Second)
		if st := p.Elements().Child("status"); st != nil {
			status = st.Text()
		}
	}
	x.sendReply(iq, secs, status)
}

func (x *LastActivity) sendReply(iq *xmpp.IQ, secs int, status string) {
	q := xmpp.NewElementNamespace("query", lastActivityNamespace)
	q.SetText(status)
	q.SetAttribute("seconds", strconv.Itoa(secs))
	res := iq.ResultIQ()
	res.AppendElement(q)
	x.stm.SendElement(res)
}
