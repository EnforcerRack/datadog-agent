// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package appsec

import (
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/log"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
)

type span struct {
	*pb.Span
}

func startSpan(traceID, parentID uint64, name, typ string) span {
	start := time.Now().UnixNano()
	spanID := generateSpanID(start)
	if traceID == 0 {
		traceID = spanID
	}
	return span{
		Span: &pb.Span{
			TraceID:  traceID,
			ParentID: parentID,
			SpanID:   spanID,
			Start:    start,
			Name:     name,
			Type:     typ,
			Meta:     map[string]string{},
			Metrics:  map[string]float64{},
		},
	}
}

func (s *span) finish() {
	s.Duration = time.Now().UnixNano() - s.Start
}

type httpSpan struct {
	span
}

func startHTTPRequestSpan(traceID, parentID uint64, resource string) httpSpan {
	sp := startSpan(traceID, parentID, "http.request", "web")
	sp.Resource = resource
	return httpSpan{sp}
}

func (s *httpSpan) SetMethod(m string)     { s.Meta["http.method"] = m }
func (s *httpSpan) SetURL(u string)        { s.Meta["http.url"] = u }
func (s *httpSpan) SetUserAgent(ua string) { s.Meta["http.useragent"] = ua }
func (s *httpSpan) SetRequestHeaders(headers map[string]string) {
	for k, v := range headers {
		s.SetRequestHeader(k, v)
	}
}
func (s *httpSpan) SetRequestHeader(header, value string) {
	s.Meta["http.request.headers."+header] = value
}
func (s *httpSpan) SetResponseHeaders(headers map[string]string) {
	for k, v := range headers {
		s.SetRequestHeader(k, v)
	}
}
func (s *httpSpan) SetResponseHeader(header, value string) {
	s.Meta["http.response.headers."+header] = value
}

// generateSpanID returns a random uint64 that has been XORd with the startTime.
// This is done to get around the 32-bit random seed limitation that may create collisions if there is a large number
// of services all generating spans.
func generateSpanID(startTime int64) uint64 {
	return random.Uint64() ^ uint64(startTime)
}

// random holds a thread-safe source of random numbers.
var random *rand.Rand

func init() {
	var seed int64
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(math.MaxInt64))
	if err == nil {
		seed = n.Int64()
	} else {
		log.Warn("cannot generate random seed: %v; using current time", err)
		seed = time.Now().UnixNano()
	}
	random = rand.New(&safeSource{
		source: rand.NewSource(seed),
	})
}

// safeSource holds a thread-safe implementation of rand.Source64.
type safeSource struct {
	source rand.Source
	sync.Mutex
}

func (rs *safeSource) Int63() int64 {
	rs.Lock()
	n := rs.source.Int63()
	rs.Unlock()

	return n
}

func (rs *safeSource) Uint64() uint64 { return uint64(rs.Int63()) }

func (rs *safeSource) Seed(seed int64) {
	rs.Lock()
	rs.source.Seed(seed)
	rs.Unlock()
}

// envClientIPHeader is the name of the env var used to specify the IP header to be used for client IP collection.
const envClientIPHeader = "DD_TRACE_CLIENT_IP_HEADER"

var (
	ipv6SpecialNetworks = []*netaddrIPPrefix{
		ippref("fec0::/10"), // site local
	}
	clientIPHeader string

	defaultIPHeaders = []string{
		"x-forwarded-for",
		"x-real-ip",
		"x-client-ip",
		"x-forwarded",
		"x-cluster-client-ip",
		"forwarded-for",
		"forwarded",
		"via",
		"true-client-ip",
	}

	// List of HTTP headers we collect and send.
	collectedHTTPHeaders = append(defaultIPHeaders,
		"host",
		"content-length",
		"content-type",
		"content-encoding",
		"content-language",
		"forwarded",
		"user-agent",
		"accept",
		"accept-encoding",
		"accept-language")
)

func init() {
	// Required by sort.SearchStrings
	sort.Strings(collectedHTTPHeaders[:])

	// Read the IP-parsing configuration
	clientIPHeader = strings.ToLower(os.Getenv(envClientIPHeader))
}

// setAppSecEnabledTags sets the AppSec-specific span tags that are expected to
// be in service entry span when AppSec is enabled.
func setAppSecEnabledTags(span span) {
	span.Metrics["_dd.appsec.enabled"] = 1
}

// setEventSpanTags sets the security event span tags into the service entry span.
func setEventSpanTags(span httpSpan, events json.RawMessage) error {
	// Set the appsec event span tag
	val, err := makeEventsTagValue(events)
	if err != nil {
		return err
	}
	span.Meta["_dd.appsec.json"] = string(val)
	// Set the appsec.event tag needed by the appsec backend
	span.Meta["appsec.event"] = "true"
	return nil
}

// Create the value of the security events tag.
func makeEventsTagValue(events json.RawMessage) (json.RawMessage, error) {
	// Create the structure to use in the `_dd.appsec.json` span tag.
	v := struct {
		Triggers json.RawMessage `json:"triggers"`
	}{Triggers: events}
	tag, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("unexpected error while serializing the appsec event span tag: %v", err)
	}
	return tag, nil
}

// setSecurityEventsTags sets the AppSec-specific span tags when security events were found.
func setSecurityEventsTags(span httpSpan, events json.RawMessage, headers, respHeaders map[string][]string) {
	if err := setEventSpanTags(span, events); err != nil {
		log.Errorf("appsec: unexpected error while creating the appsec event tags: %v", err)
		return
	}
	span.SetRequestHeaders(normalizeHTTPHeaders(headers))
	span.SetResponseHeaders(normalizeHTTPHeaders(respHeaders))
}

// normalizeHTTPHeaders returns the HTTP headers following Datadog's
// normalization format.
func normalizeHTTPHeaders(headers map[string][]string) (normalized map[string]string) {
	if len(headers) == 0 {
		return nil
	}
	normalized = make(map[string]string)
	for k, v := range headers {
		k = strings.ToLower(k)
		if i := sort.SearchStrings(collectedHTTPHeaders[:], k); i < len(collectedHTTPHeaders) && collectedHTTPHeaders[i] == k {
			normalized[k] = strings.Join(v, ",")
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// ippref returns the IP network from an IP address string s. If not possible, it returns nil.
func ippref(s string) *netaddrIPPrefix {
	if prefix, err := netaddrParseIPPrefix(s); err == nil {
		return &prefix
	}
	return nil
}

// setClientIPTags sets the http.client_ip, http.request.headers.*, and
// network.client.ip span tags according to the request headers and remote
// connection address. Note that the given request headers reqHeaders must be
// normalized with lower-cased keys for this function to work.
func setClientIPTags(span httpSpan, remoteAddr string, reqHeaders map[string][]string) {
	ipHeaders := defaultIPHeaders
	if len(clientIPHeader) > 0 {
		ipHeaders = []string{clientIPHeader}
	}

	var (
		headers []string
		ips     []string
	)
	for _, hdr := range ipHeaders {
		if v, _ := reqHeaders[hdr]; len(v) > 0 {
			headers = append(headers, hdr)
			ips = append(ips, v...)
		}
	}

	var remoteIP netaddrIP
	if remoteAddr != "" {
		remoteIP = parseIP(remoteAddr)
		if remoteIP.IsValid() {
			span.Meta["network.client.ip"] = remoteIP.String()
		}
	}

	switch len(ips) {
	case 0:
		ip := remoteIP.String()
		if remoteIP.IsValid() && isGlobal(remoteIP) {
			span.Meta["http.client_ip"] = ip
		}
	case 1:
		for _, ipstr := range strings.Split(ips[0], ",") {
			ip := parseIP(strings.TrimSpace(ipstr))
			if ip.IsValid() && isGlobal(ip) {
				span.Meta["http.client_ip"] = ip.String()
				break
			}
		}
	default:
		for _, hdr := range headers {
			span.SetRequestHeader(hdr, strings.Join(reqHeaders[hdr], ","))
		}
		span.Meta["_dd.multiple-ip-headers"] = strings.Join(headers, ",")
	}
}

func parseIP(s string) netaddrIP {
	if ip, err := netaddrParseIP(s); err == nil {
		return ip
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		if ip, err := netaddrParseIP(h); err == nil {
			return ip
		}
	}
	return netaddrIP{}
}

func isGlobal(ip netaddrIP) bool {
	// IsPrivate also checks for ipv6 ULA.
	// We care to check for these addresses are not considered public, hence not global.
	// See https://www.rfc-editor.org/rfc/rfc4193.txt for more details.
	isGlobal := !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
	if !isGlobal || !ip.Is6() {
		return isGlobal
	}
	for _, n := range ipv6SpecialNetworks {
		if n.Contains(ip) {
			return false
		}
	}
	return isGlobal
}

type netaddrIP = netip.Addr
type netaddrIPPrefix = netip.Prefix

var (
	netaddrParseIP       = netip.ParseAddr
	netaddrParseIPPrefix = netip.ParsePrefix
	netaddrMustParseIP   = netip.MustParseAddr
	netaddrIPv6Raw       = netip.AddrFrom16
)

func netaddrIPv4(a, b, c, d byte) netaddrIP {
	e := [4]byte{a, b, c, d}
	return netip.AddrFrom4(e)
}
