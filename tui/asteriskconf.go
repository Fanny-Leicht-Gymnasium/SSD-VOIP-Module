package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// asteriskDir is the directory docker-compose.yml mounts into the asterisk
// container (./asterisk relative to the project root).
func asteriskDir() string {
	return filepath.Join(projectDir(), "asterisk")
}

// writeAsteriskConfig renders ari.conf, http.conf, pjsip.conf, and
// extensions.conf from the current field values and writes them into
// asterisk/. This keeps the SIP trunk credentials, ARI user/password, ARI
// app name, and PJSIP endpoint name in sync between the TUI/.env and the
// actual Asterisk config, instead of those values being hardcoded twice.
func writeAsteriskConfig(values map[string]string) error {
	dir := asteriskDir()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	files := map[string]string{
		"http.conf":       httpConf(),
		"ari.conf":        ariConf(values),
		"pjsip.conf":      pjsipConf(values),
		"extensions.conf": extensionsConf(values),
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	return nil
}

func httpConf() string {
	return `[general]
enabled=yes
bindaddr=0.0.0.0
bindport=8088
`
}

func ariConf(values map[string]string) string {
	return fmt.Sprintf(`[general]
enabled=yes
pretty=yes
allowed_origins = *

[%s]
type=user
read_only=no
password=%s
`, values["ARI_USER"], values["ARI_PASSWORD"])
}

func pjsipConf(values map[string]string) string {
	endpoint := values["CALL_ENDPOINT"]

	return fmt.Sprintf(`; =========================
; TRANSPORT
; =========================
[transport-udp]
type=transport
protocol=udp
bind=0.0.0.0:5060

; =========================
; GLOBAL OPTIONS
; =========================
[global]
type=global
endpoint_identifier_order=auth_username,ip,username,anonymous

; =========================
; SIP TRUNK AUTH
; =========================
[%s-auth]
type=auth
auth_type=userpass
username=%s
password=%s

; =========================
; SIP TRUNK AOR
; =========================
[%s-aor]
type=aor
contact=sip:%s

; =========================
; SIP TRUNK ENDPOINT
; =========================
[%s]
type=endpoint
transport=transport-udp
context=default

disallow=all
allow=alaw,ulaw

outbound_auth=%s-auth
aors=%s-aor

from_user=%s
from_domain=%s

direct_media=no
rtp_symmetric=yes
force_rport=yes
rewrite_contact=yes

dtmf_mode=rfc4733

; =========================
; IDENTIFY
; =========================
[%s-identify]
type=identify
endpoint=%s
match=%s

send_pai=yes
trust_id_inbound=yes
`,
		endpoint, values["SIP_USERNAME"], values["SIP_PASSWORD"],
		endpoint, values["SIP_DOMAIN"],
		endpoint,
		endpoint, endpoint,
		values["SIP_USERNAME"], values["SIP_DOMAIN"],
		endpoint, endpoint, values["SIP_DOMAIN"],
	)
}

func extensionsConf(values map[string]string) string {
	app := values["APP_NAME"]

	return fmt.Sprintf(`[general]
static=yes
writeprotect=no
clearglobalvars=no

[globals]

; =========================
; DEFAULT CONTEXT
; Matches pjsip.conf's endpoint (context=default). Hands the channel
; to the ARI app so the bot controls playback / DTMF over ARI + the
; module WebSocket, instead of the dialplan doing it.
; =========================
[default]
exten => _X.,1,NoOp(Incoming call to ${EXTEN})
 same => n,Answer()
 same => n,Stasis(%s,${EXTEN})
 same => n,Hangup()

; Feature codes / short codes starting with '*' (e.g. TARGET_NUMBER=**621)
exten => _*X.,1,NoOp(Incoming feature-code call to ${EXTEN})
 same => n,Answer()
 same => n,Stasis(%s,${EXTEN})
 same => n,Hangup()

exten => i,1,Hangup()
exten => t,1,Hangup()

; =========================
; OUTGOING CONTEXT
; Used when the bot originates a call against TARGET_NUMBER.
; =========================
[outgoing]
include => default
`, app, app)
}