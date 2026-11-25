import java.util.*;

// Records java.util.HashSet iteration orders for the Go tests. Keys are
// points with the hashCode Eclipse generates (31 * (31 + x) + y), as in
// several referees. Output, per case:
//
//   case <n>
//   a <x> <y>        add a point (duplicates included)
//   c                clear
//   o <x>,<y> ...    the iteration order at this point
public class Gen {
    static final class P {
        final int x, y;
        P(int x, int y) { this.x = x; this.y = y; }
        @Override public int hashCode() { return 31 * (31 + x) + y; }
        @Override public boolean equals(Object o) { return o instanceof P && ((P) o).x == x && ((P) o).y == y; }
    }

    public static void main(String[] args) {
        Random r = new Random(20261125L);
        StringBuilder sb = new StringBuilder();
        for (int c = 0; c < 60; c++) {
            sb.append("case ").append(c).append('\n');
            // Small grids like Hypersonic's, and wider ones whose hashes
            // collide more, so buckets fill up and force resizes.
            int w = 1 + r.nextInt(c % 3 == 0 ? 60 : 15), h = 1 + r.nextInt(c % 3 == 0 ? 25 : 13);
            Set<P> set = new HashSet<>();
            int rounds = 1 + r.nextInt(4);
            for (int k = 0; k < rounds; k++) {
                int adds = r.nextInt(Math.min(w * h, 150) + 1);
                for (int i = 0; i < adds; i++) {
                    int x = r.nextInt(w), y = r.nextInt(h);
                    set.add(new P(x, y));
                    sb.append("a ").append(x).append(' ').append(y).append('\n');
                }
                sb.append('o');
                for (P p : set) sb.append(' ').append(p.x).append(',').append(p.y);
                sb.append('\n');
                if (k + 1 < rounds) {
                    set.clear();
                    sb.append("c\n");
                }
            }
        }
        System.out.print(sb);
    }
}
