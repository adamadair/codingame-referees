import java.io.*;
import java.util.*;

// Prints Java's results for the Go tests, as raw IEEE-754 bits in hex:
//
//   atan2 <y bits> <x bits> <Math.atan2 bits>
//   round <a bits> <Math.round> <(long) a> <(int) a>
//
// Inputs: grids of small and half-integer values (the hex-grid angles
// that game referees compute), special values, and seeded random values
// across all magnitudes. When this was generated with JDK 21, Math.atan2
// and StrictMath.atan2 (fdlibm) agreed on every input.
public class Gen {
    static String h(double d) { return Long.toHexString(Double.doubleToRawLongBits(d)); }

    static void atan2(PrintStream out, double y, double x) {
        out.println("atan2 " + h(y) + " " + h(x) + " " + h(Math.atan2(y, x)));
    }

    static void round(PrintStream out, double a) {
        out.println("round " + h(a) + " " + Math.round(a) + " " + (long) a + " " + (int) a);
    }

    public static void main(String[] args) {
        PrintStream out = System.out;
        double[] special = { 0.0, -0.0, 1.0, -1.0, 0.5, -0.5, Double.MIN_VALUE, -Double.MIN_VALUE, Double.MAX_VALUE,
                -Double.MAX_VALUE, Double.POSITIVE_INFINITY, Double.NEGATIVE_INFINITY, Double.NaN, 1e-310, 3.0, 1e18, -1e300 };

        // Hex-grid deltas: dy = k*sqrt(3)/2, dx in half steps, plus integers.
        for (int k = -20; k <= 20; k++) {
            for (int j = -46; j <= 46; j++) {
                atan2(out, k * Math.sqrt(3) / 2, j * 0.5);
            }
        }
        for (double y : special) for (double x : special) atan2(out, y, x);

        Random r = new Random(20261125L);
        for (int i = 0; i < 1500; i++) {
            double y = (r.nextDouble() - 0.5) * Math.pow(2, r.nextInt(200) - 100);
            double x = (r.nextDouble() - 0.5) * Math.pow(2, r.nextInt(200) - 100);
            atan2(out, y, x);
        }
        for (int i = 0; i < 800; i++) { // large integer deltas, like off-map targets
            atan2(out, (r.nextInt(400000001) - 200000000) * Math.sqrt(3) / 2, (r.nextInt(400000001) - 200000000) + 0.5 * r.nextInt(2));
        }

        double[] rounds = { 0.5, -0.5, 1.5, -1.5, 2.5, -2.5, 0.49999999999999994, -0.49999999999999994, 4503599627370495.5,
                -4503599627370495.5, 4503599627370497.0, 9.223372036854776E18, -9.223372036854776E18, 1e19, -1e19, 2147483647.5,
                2147483648.0, -2147483648.5, 1e-320 };
        for (double a : rounds) round(out, a);
        for (double a : special) round(out, a);
        for (int i = 0; i < 400; i++) {
            round(out, (r.nextDouble() - 0.5) * Math.pow(2, r.nextInt(140) - 70));
            round(out, (r.nextInt(2001) - 1000) / 3.0);
            round(out, (r.nextInt(2001) - 1000) / 2.0);
        }
    }
}
