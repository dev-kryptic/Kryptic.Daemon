import AppKit
import Darwin
import Foundation

enum SingleInstanceGuard {
    private static let lockPath = (NSTemporaryDirectory() as NSString)
        .appendingPathComponent("dev.kryptic.daemon.lock")
    private static var lockFD: Int32 = -1

    static func acquire() -> Bool {
        lockFD = open(lockPath, O_CREAT | O_RDWR, 0644)
        guard lockFD >= 0 else { return true }
        return flock(lockFD, LOCK_EX | LOCK_NB) == 0
    }
}
