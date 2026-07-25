#!/bin/bash
# wifi-watchdog.sh — long-running daemon (Type=simple, Restart=always) that recovers
# a wedged Pi Zero W wlan0 without a physical power-cycle. Checks LAN reachability
# every ${INTERVAL}s.
#
# Logs ONLY meaningful events (drops, recoveries) + a once-an-hour healthy heartbeat,
# so the journal stays a compact, readable incident history (and light on SD writes) —
# no per-minute systemd framework spam.
#
# SAFE ON THE ROAD: only reloads the driver / reboots when the WiFi *radio* looks broken
# (NM state 'unavailable' / iface gone) OR our home network is in range but unreachable.
# When the radio is fine and home SSID is simply out of range (driving), it does nothing
# — NetworkManager reconnects on its own once a known network reappears.
set -u

IFACE=wlan0
HOME_SSID=Kolliefiets7                                 # home network; "unreachable" != "in range"
STATE=/run/wifi-watchdog.fails                         # tmpfs; for external `cat` inspection
REBOOT_STAMP=/var/lib/wifi-watchdog/last-reboot        # persists across reboots
INTERVAL=60                                            # seconds between checks
HEARTBEAT=3600                                         # log a healthy heartbeat at most this often

SOFT=3                 # ~3 min down -> nudge NetworkManager
HARD=5                 # ~5 min down -> reload brcmfmac driver
REBOOT=8               # ~8 min down -> reboot as last resort
REBOOT_COOLDOWN=1800   # never auto-reboot more than once per 30 min

nm_state()      { nmcli -t -f DEVICE,STATE device status 2>/dev/null | awk -F: -v i="$IFACE" '$1==i{print $2}'; }
home_in_range() { nmcli -t -f SSID device wifi list 2>/dev/null | grep -Fxq "$HOME_SSID"; }
log()           { echo "wifi-watchdog: $*"; }

log "started (interval ${INTERVAL}s, gateway target, home SSID '$HOME_SSID')"

fails=0
last_hb=0
while :; do
    GW=$(ip route show default 2>/dev/null | awk '/default/{print $3; exit}')
    TARGET="${GW:-192.168.178.1}"                       # LAN gateway, fallback to known IP
    now=$(date +%s)

    if ping -c 2 -W 2 "$TARGET" >/dev/null 2>&1; then
        if [ "$fails" -ne 0 ]; then
            log "link OK again after $fails failed check(s) (~$fails min)"
            fails=0
        fi
        if [ $((now - last_hb)) -ge "$HEARTBEAT" ]; then
            log "heartbeat — healthy, gateway $TARGET reachable"
            last_hb=$now
        fi
        echo "$fails" > "$STATE"
        sleep "$INTERVAL"; continue
    fi

    fails=$((fails + 1))
    echo "$fails" > "$STATE"

    state=$(nm_state)
    radio_dead=0
    [ -e /sys/class/net/$IFACE ] || radio_dead=1       # interface vanished
    [ "$state" = "unavailable" ] && radio_dead=1       # driver/radio unusable per NM

    if [ "$radio_dead" -eq 0 ] && ! home_in_range; then
        # travelling / out of range — log the transition once, then stay quiet
        [ "$fails" -eq 1 ] && log "gateway unreachable, radio OK (state=$state), '$HOME_SSID' not in range — travelling/out of range, holding (will not reboot)"
        sleep "$INTERVAL"; continue
    fi

    if [ "$radio_dead" -eq 1 ]; then reason="radio dead (state=${state:-none})"; else reason="home in range but unreachable"; fi
    log "recovering — $reason (fails=$fails)"

    if   [ "$fails" -ge "$REBOOT" ]; then
        last=$(cat "$REBOOT_STAMP" 2>/dev/null || echo 0)
        if [ $((now - last)) -ge "$REBOOT_COOLDOWN" ]; then
            log "rebooting as last resort"
            mkdir -p "$(dirname "$REBOOT_STAMP")"; echo "$now" > "$REBOOT_STAMP"
            systemctl reboot
        else
            log "reboot suppressed — within ${REBOOT_COOLDOWN}s cooldown"
        fi
    elif [ "$fails" -eq "$HARD" ]; then
        log "reloading brcmfmac driver"
        ip link set "$IFACE" down 2>/dev/null
        modprobe -r brcmfmac 2>/dev/null
        sleep 3
        modprobe brcmfmac 2>/dev/null
        systemctl restart NetworkManager
    elif [ "$fails" -eq "$SOFT" ]; then
        log "nudging $IFACE via NetworkManager"
        nmcli device reconnect "$IFACE" 2>/dev/null || systemctl restart NetworkManager
    fi
    sleep "$INTERVAL"
done
