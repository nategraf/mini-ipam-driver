package bytop

func max(x, y int) int {
    if x > y {
        return x
    }
    return y
}

func Not(a, dst []byte) []byte {
    if dst == nil {
        dst = make([]byte, len(a))
    }
    for i, _ := range dst {
        dst[i] = ^a[i]
    }
    return dst
}

func And(a, b, dst []byte) []byte {
    if dst == nil {
        dst = make([]byte, max(len(a), len(b)))
    }
    for i, _ := range dst {
        dst[i] = 0xFF
        if i < len(a) {
            dst[i] &= a[i]
        }
        if i < len(b) {
            dst[i] &= b[i]
        }
    }
    return dst
}

func Or(a, b, dst []byte) []byte {
    if dst == nil {
        dst = make([]byte, max(len(a), len(b)))
    }
    for i, _ := range dst {
        dst[i] = 0x00
        if i < len(a) {
            dst[i] |= a[i]
        }
        if i < len(b) {
            dst[i] |= b[i]
        }
    }
    return dst
}

func Add(a []byte, n int32, dst []byte) []byte {
    if dst == nil {
        dst = make([]byte, len(a))
    }

    // Assume big-endian (network order) as that is how net.IP is arranged
    carry := uint32(0)
    for i := uint32(len(dst)-1); i >= 0; i-- {
        carry += uint32(int32(a[i]) + n)
        dst[i], carry = byte(carry % 0xFF), carry / 0xFF
    }

    return dst
}

func Equal(a, b []byte) bool {
    if len(a) != len(b) {
        return false
    }

    for i, ai := range a {
        if ai != b[i] {
            return false
        }
    }

    return true
}

// Flips a bit at the index, which is from left to right (most signifcant to least)
func FlipBit(index int, s []byte) {
    i, j := index / 8, index % 8
    s[i] ^= 1 << uint(7 - j)
}
