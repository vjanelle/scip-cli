fn greet(name: []const u8) usize {
    return name.len;
}

pub fn main() void {
    _ = greet("zig");
}
