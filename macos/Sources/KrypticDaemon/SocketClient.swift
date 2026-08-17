import Darwin
import Foundation

/// Minimal client for the daemon's NDJSON unix socket (PROTOCOL.md v1).
/// One request per connection, mirroring what the SDKs do.
enum SocketClient {
    struct DaemonStatus: Equatable {
        var running = false
        var authenticated = false
        var email: String?
        var organization: String?
        var daemonVersion: String?
    }

    static var socketPath: String {
        ProcessInfo.processInfo.environment["KRYPTIC_SOCKET_PATH"] ?? "/tmp/kryptic-daemon.sock"
    }

    static func status() -> DaemonStatus {
        guard let response = request(["v": 1, "type": "status"]) else {
            return DaemonStatus()
        }
        return DaemonStatus(
            running: true,
            authenticated: response["authenticated"] as? Bool ?? false,
            email: response["email"] as? String,
            organization: response["organization"] as? String,
            daemonVersion: response["daemonVersion"] as? String
        )
    }

    /// Asks the daemon to drop its in-memory secrets cache. Returns the number of
    /// cached bundles that were cleared, or nil if the daemon was unreachable.
    static func flushSecretsCache() -> Int? {
        guard let response = request(["v": 1, "type": "flush"]),
              response["ok"] as? Bool == true else { return nil }
        return response["cleared"] as? Int ?? 0
    }

    private static func request(_ payload: [String: Any]) -> [String: Any]? {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { return nil }
        defer { close(fd) }

        var timeout = timeval(tv_sec: 2, tv_usec: 0)
        setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))
        setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))

        var address = sockaddr_un()
        address.sun_family = sa_family_t(AF_UNIX)
        let connected = socketPath.withCString { path -> Bool in
            _ = withUnsafeMutableBytes(of: &address.sun_path) { buffer in
                strlcpy(buffer.baseAddress!.assumingMemoryBound(to: CChar.self), path, buffer.count)
            }
            return withUnsafePointer(to: &address) { pointer in
                pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                    connect(fd, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) == 0
                }
            }
        }
        guard connected else { return nil }

        guard var data = try? JSONSerialization.data(withJSONObject: payload) else { return nil }
        data.append(0x0A)
        let sent = data.withUnsafeBytes { write(fd, $0.baseAddress, data.count) }
        guard sent == data.count else { return nil }

        var received = Data()
        var buffer = [UInt8](repeating: 0, count: 4096)
        while !received.contains(0x0A) && received.count < 1 << 20 {
            let count = read(fd, &buffer, buffer.count)
            guard count > 0 else { break }
            received.append(contentsOf: buffer[0..<count])
        }
        guard let newline = received.firstIndex(of: 0x0A) else { return nil }
        return (try? JSONSerialization.jsonObject(with: received[..<newline])) as? [String: Any]
    }
}
