"""Sample target program for the guyide e2e debug flow.

The e2e test sets a breakpoint inside ``add`` and asserts that, when the
debugger stops, the local variables ``a`` and ``b`` are reported by
``guyide debug state --vars --frames --json``.

Keep this file deliberately boring; the line numbers below are referenced
by the test, so don't reorder or insert lines without updating
``test_breakpoint_flow.py``.
"""


def add(a, b):  # line 13
    result = a + b  # line 14 -- breakpoint target
    return result


def main():
    total = 0
    for i in range(3):
        total = add(total, i + 1)
    print(f"total={total}")


if __name__ == "__main__":
    main()
