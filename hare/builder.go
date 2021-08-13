package hare

import (
	"encoding/hex"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/signing"
)

// Message is the tuple of a message and its corresponding signature.
type Message struct {
	Sig      []byte
	InnerMsg *InnerMessage
}

// MessageFromBuffer builds an Hare message from the provided bytes buffer.
// It returns an error if unmarshal of the provided byte slice failed.
func MessageFromBuffer(buf []byte) (*Message, error) {
	msg := &Message{}
	return msg, types.BytesToInterface(buf, msg)
}

func (m *Message) String() string {
	sig := hex.EncodeToString(m.Sig)
	l := len(sig)
	if l > 5 {
		l = 5
	}
	return fmt.Sprintf("Sig: %v… InnerMsg: %v", sig[:l], m.InnerMsg.String())
}

// Field returns a log field. Implements the LoggableField interface.
func (m *Message) Field() log.Field {
	return log.String("message", m.String())
}

// Certificate is a collection of messages and the set of values.
// Typically used as a collection of commit messages.
type Certificate struct {
	Values  []types.BlockID // the committed set S
	AggMsgs *AggregatedMessages
}

// AggregatedMessages is a collection of messages.
type AggregatedMessages struct {
	Messages []*Message
}

// InnerMessage is the actual set of fields that describe a message in the Hare protocol.
type InnerMessage struct {
	Type             MessageType
	InstanceID       types.LayerID
	K                int32 // the round counter
	Ki               int32
	Values           []types.BlockID     // the set S. optional for commit InnerMsg in a certificate
	RoleProof        []byte              // role is implicit by InnerMsg type, this is the proof
	EligibilityCount uint16              // the number of claimed eligibilities
	Svp              *AggregatedMessages // optional. only for proposal Messages
	Cert             *Certificate        // optional
}

// Bytes returns the message as bytes.
func (im *InnerMessage) Bytes() []byte {
	buf, err := types.InterfaceToBytes(im)
	if err != nil {
		panic("could not marshal InnerMsg before send")
	}
	return buf
}

func (im *InnerMessage) String() string {
	return fmt.Sprintf("Type: %v InstanceID: %v K: %v Ki: %v", im.Type, im.InstanceID, im.K, im.Ki)
}

// messageBuilder is the impl of the builder DP.
// It allows the user to set the different fields of the builder and eventually Build the message.
type messageBuilder struct {
	msg   *Msg
	inner *InnerMessage
}

// newMessageBuilder returns a new, empty message builder.
// One should not assume any values are pre-set.
func newMessageBuilder() *messageBuilder {
	m := &messageBuilder{&Msg{Message: &Message{}, PubKey: nil}, &InnerMessage{}}
	m.msg.InnerMsg = m.inner

	return m
}

// Build returns the protocol message as type Msg.
func (builder *messageBuilder) Build() *Msg {
	return builder.msg
}

func (builder *messageBuilder) SetCertificate(certificate *Certificate) *messageBuilder {
	builder.inner.Cert = certificate
	return builder
}

// Sign calls the provided signer to calculate the signature and then set it accordingly.
func (builder *messageBuilder) Sign(signing Signer) *messageBuilder {
	builder.msg.Sig = signing.Sign(builder.inner.Bytes())

	return builder
}

// SetPubKey sets the public key of the message.
// Note: the message itself does not contain the public key. The builder returns the wrapper of the message which does.
func (builder *messageBuilder) SetPubKey(pub *signing.PublicKey) *messageBuilder {
	builder.msg.PubKey = pub
	return builder
}

func (builder *messageBuilder) SetType(msgType MessageType) *messageBuilder {
	builder.inner.Type = msgType
	return builder
}

func (builder *messageBuilder) SetInstanceID(id types.LayerID) *messageBuilder {
	builder.inner.InstanceID = id
	return builder
}

func (builder *messageBuilder) SetRoundCounter(k int32) *messageBuilder {
	builder.inner.K = k
	return builder
}

func (builder *messageBuilder) SetKi(ki int32) *messageBuilder {
	builder.inner.Ki = ki
	return builder
}

func (builder *messageBuilder) SetValues(set *Set) *messageBuilder {
	builder.inner.Values = set.ToSlice()
	return builder
}

func (builder *messageBuilder) SetRoleProof(sig []byte) *messageBuilder {
	builder.inner.RoleProof = sig
	return builder
}

func (builder *messageBuilder) SetEligibilityCount(eligibilityCount uint16) *messageBuilder {
	builder.inner.EligibilityCount = eligibilityCount
	return builder
}

func (builder *messageBuilder) SetSVP(svp *AggregatedMessages) *messageBuilder {
	builder.inner.Svp = svp
	return builder
}
