// Package periodic publishes frames on one interface.
// The caller decides which frame is sent and how often. A receipt is the
// local time of a completed write on the receive clock. It is not evidence
// of transmission on the wire or of an equipment effect.
package periodic
