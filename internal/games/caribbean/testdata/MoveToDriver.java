import java.util.Random;

// Test driver for the Go port's autopilot tests (moveto_test.go). It is
// compiled at test time together with the official Coders of the Caribbean
// Referee.java, whose classes it calls; no referee code is copied here.
//
// Modes:
//   moveto  one line per (x, y, orientation, speed) on the map, with one
//           character per on-map target (y-major): '.' for no action,
//           otherwise 'A' + Action.ordinal().
//   offmap  "x y orientation speed tx ty action" for seeded random cases
//           with targets far off the map.
//   angle   one line per on-map cell with Coord.angle() to every on-map
//           cell, as raw double bits in hex, space separated.
public class MoveToDriver {
    static final int W = 23, H = 21;

    static char action(int x, int y, int o, int speed, int tx, int ty) {
        Referee.Ship ship = new Referee.Ship(x, y, o, 0);
        ship.speed = speed;
        ship.moveTo(tx, ty);
        return ship.action == null ? '.' : (char) ('A' + ship.action.ordinal());
    }

    public static void main(String[] args) {
        StringBuilder sb = new StringBuilder(1 << 20);
        switch (args[0]) {
        case "moveto":
            for (int y = 0; y < H; y++)
                for (int x = 0; x < W; x++)
                    for (int o = 0; o < 6; o++)
                        for (int s = 0; s <= 2; s++) {
                            for (int ty = 0; ty < H; ty++)
                                for (int tx = 0; tx < W; tx++)
                                    sb.append(action(x, y, o, s, tx, ty));
                            sb.append('\n');
                        }
            break;
        case "offmap":
            Random r = new Random(Long.parseLong(args[1]));
            for (int i = 0; i < 20000; i++) {
                int x = r.nextInt(W), y = r.nextInt(H), o = r.nextInt(6), s = r.nextInt(3);
                int tx, ty;
                if (i % 2 == 0) {
                    tx = r.nextInt(199999999) - 99999999;
                    ty = r.nextInt(199999999) - 99999999;
                } else {
                    tx = r.nextInt(W + 20) - 10;
                    ty = r.nextInt(H + 20) - 10;
                }
                sb.append(x).append(' ').append(y).append(' ').append(o).append(' ').append(s).append(' ')
                        .append(tx).append(' ').append(ty).append(' ').append(action(x, y, o, s, tx, ty)).append('\n');
            }
            break;
        case "angle":
            for (int y = 0; y < H; y++)
                for (int x = 0; x < W; x++) {
                    Referee.Coord from = new Referee.Coord(x, y);
                    for (int ty = 0; ty < H; ty++)
                        for (int tx = 0; tx < W; tx++) {
                            if (tx + ty > 0) sb.append(' ');
                            sb.append(Long.toHexString(Double.doubleToRawLongBits(from.angle(new Referee.Coord(tx, ty)))));
                        }
                    sb.append('\n');
                }
            break;
        default:
            throw new IllegalArgumentException(args[0]);
        }
        System.out.print(sb);
    }
}
