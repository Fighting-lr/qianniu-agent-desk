"use client"

export type RealtimeConnectionStatus = "connecting" | "connected" | "disconnected"

type RealtimeConnectionManagerOptions = {
  createSocket: () => WebSocket
  canReconnect?: () => boolean
  onStatusChange?: (status: RealtimeConnectionStatus) => void
  onSocketChange?: (socket: WebSocket | null) => void
  onOpen?: (socket: WebSocket) => void
  onMessage?: (event: MessageEvent, socket: WebSocket) => void
  onClose?: (event: CloseEvent, socket: WebSocket) => void
  onError?: (event: Event, socket: WebSocket) => void
  onConnectError?: (error: unknown) => void
  buildPingMessage?: () => string
  pingIntervalMs?: number
  reconnectBaseDelayMs?: number
  reconnectMaxDelayMs?: number
}

type DisconnectOptions = {
  reconnect?: boolean
  updateStatus?: boolean
}

const DEFAULT_PING_INTERVAL_MS = 20000
const DEFAULT_RECONNECT_BASE_DELAY_MS = 2000
const DEFAULT_RECONNECT_MAX_DELAY_MS = 30000

export function createRealtimeConnectionManager(
  options: RealtimeConnectionManagerOptions
) {
  let socket: WebSocket | null = null
  let reconnectTimer: number | null = null
  let pingTimer: number | null = null
  let reconnectAttempt = 0
  let reconnectEnabled = false

  const clearReconnectTimer = () => {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  const clearPingTimer = () => {
    if (pingTimer !== null) {
      window.clearInterval(pingTimer)
      pingTimer = null
    }
  }

  const clearTimers = () => {
    clearReconnectTimer()
    clearPingTimer()
  }

  const setSocket = (nextSocket: WebSocket | null) => {
    socket = nextSocket
    options.onSocketChange?.(nextSocket)
  }

  const canReconnect = () => {
    return reconnectEnabled && (options.canReconnect?.() ?? true)
  }

  const scheduleReconnect = () => {
    if (!canReconnect() || reconnectTimer !== null) {
      return
    }

    const delay = Math.min(
      (options.reconnectBaseDelayMs ?? DEFAULT_RECONNECT_BASE_DELAY_MS) *
        2 ** reconnectAttempt,
      options.reconnectMaxDelayMs ?? DEFAULT_RECONNECT_MAX_DELAY_MS
    )
    options.onStatusChange?.("connecting")
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      reconnectAttempt += 1
      if (canReconnect()) {
        connect()
      }
    }, delay)
  }

  const connect = () => {
    reconnectEnabled = true
    if (!(options.canReconnect?.() ?? true)) {
      return
    }

    disconnect({ reconnect: true, updateStatus: false })
    reconnectEnabled = true
    options.onStatusChange?.("connecting")

    let nextSocket: WebSocket
    try {
      nextSocket = options.createSocket()
    } catch (error) {
      options.onStatusChange?.("disconnected")
      options.onConnectError?.(error)
      scheduleReconnect()
      return
    }

    setSocket(nextSocket)

    nextSocket.addEventListener("open", () => {
      if (socket !== nextSocket) {
        return
      }
      clearReconnectTimer()
      clearPingTimer()
      reconnectAttempt = 0
      options.onStatusChange?.("connected")
      const buildPingMessage =
        options.buildPingMessage ?? (() => JSON.stringify({ type: "ping" }))
      pingTimer = window.setInterval(() => {
        if (nextSocket.readyState === WebSocket.OPEN) {
          nextSocket.send(buildPingMessage())
        }
      }, options.pingIntervalMs ?? DEFAULT_PING_INTERVAL_MS)
      options.onOpen?.(nextSocket)
    })

    nextSocket.addEventListener("message", (event) => {
      if (socket === nextSocket) {
        options.onMessage?.(event, nextSocket)
      }
    })

    nextSocket.addEventListener("close", (event) => {
      clearPingTimer()
      const isCurrentSocket = socket === nextSocket
      if (isCurrentSocket) {
        setSocket(null)
      }
      options.onClose?.(event, nextSocket)
      if (!isCurrentSocket) {
        return
      }
      if (canReconnect()) {
        scheduleReconnect()
      } else {
        options.onStatusChange?.("disconnected")
      }
    })

    nextSocket.addEventListener("error", (event) => {
      if (socket !== nextSocket) {
        return
      }
      options.onError?.(event, nextSocket)
      options.onStatusChange?.("disconnected")
      scheduleReconnect()
    })
  }

  const disconnect = (disconnectOptions?: DisconnectOptions) => {
    reconnectEnabled = disconnectOptions?.reconnect ?? false
    clearTimers()
    if (!reconnectEnabled) {
      reconnectAttempt = 0
    }

    const currentSocket = socket
    setSocket(null)
    if (
      currentSocket &&
      (currentSocket.readyState === WebSocket.OPEN ||
        currentSocket.readyState === WebSocket.CONNECTING)
    ) {
      currentSocket.close()
    }
    if (disconnectOptions?.updateStatus ?? !reconnectEnabled) {
      options.onStatusChange?.("disconnected")
    }
  }

  return {
    connect,
    disconnect,
    reconnect: () => {
      reconnectEnabled = true
      connect()
    },
    getSocket: () => socket,
  }
}
