import java.util.Random;

public class Gen {
    public static void main(String[] a) {
        long[] seeds = {0L, 42L, -1L, 123456789L, Long.MIN_VALUE};
        for (long s : seeds) {
            Random r = new Random(s);
            StringBuilder sb = new StringBuilder();
            sb.append("seed ").append(s).append('\n');
            sb.append("int");      for (int i = 0; i < 5; i++) sb.append(' ').append(r.nextInt()); sb.append('\n');
            sb.append("int10");    for (int i = 0; i < 5; i++) sb.append(' ').append(r.nextInt(10)); sb.append('\n');
            sb.append("int16");    for (int i = 0; i < 5; i++) sb.append(' ').append(r.nextInt(16)); sb.append('\n');
            sb.append("intbig");   for (int i = 0; i < 5; i++) sb.append(' ').append(r.nextInt(1_500_000_001)); sb.append('\n');
            sb.append("long");     for (int i = 0; i < 5; i++) sb.append(' ').append(r.nextLong()); sb.append('\n');
            sb.append("bool");     for (int i = 0; i < 5; i++) sb.append(' ').append(r.nextBoolean()); sb.append('\n');
            sb.append("double");   for (int i = 0; i < 5; i++) sb.append(' ').append(Long.toHexString(Double.doubleToRawLongBits(r.nextDouble()))); sb.append('\n');
            sb.append("float");    for (int i = 0; i < 5; i++) sb.append(' ').append(Integer.toHexString(Float.floatToRawIntBits(r.nextFloat()))); sb.append('\n');
            sb.append("gauss");    for (int i = 0; i < 6; i++) sb.append(' ').append(Long.toHexString(Double.doubleToRawLongBits(r.nextGaussian()))); sb.append('\n');
            System.out.print(sb);
        }
    }
}
