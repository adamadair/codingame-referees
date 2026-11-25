import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.*;

// Reads hex-encoded UTF-8 inputs (one per line) from inputs.txt and prints,
// for each, what Java does with it:
//
//   case <n>
//   int <value> | int error
//   long <value> | long error
//   split= <count> <hex parts...>
//   split_ <count> <hex parts...>      (split on a space)
public class Gen {
    static String hex(String s) {
        StringBuilder sb = new StringBuilder("x");
        for (byte b : s.getBytes(StandardCharsets.UTF_8)) sb.append(String.format("%02x", b));
        return sb.toString();
    }

    static String split(String s, String sep) {
        String[] parts = s.split(sep);
        StringBuilder sb = new StringBuilder(String.valueOf(parts.length));
        for (String p : parts) sb.append(' ').append(hex(p));
        return sb.toString();
    }

    public static void main(String[] a) throws IOException {
        List<String> lines = new ArrayList<>();
        try (BufferedReader r = new BufferedReader(new FileReader(a[0], StandardCharsets.UTF_8))) {
            for (String l; (l = r.readLine()) != null;) lines.add(l);
        }
        PrintStream out = new PrintStream(System.out, true, StandardCharsets.UTF_8);
        for (int i = 0; i < lines.size(); i++) {
            String h = lines.get(i).trim();
            byte[] b = new byte[h.length() / 2];
            for (int j = 0; j < b.length; j++) b[j] = (byte) Integer.parseInt(h.substring(2 * j, 2 * j + 2), 16);
            String in = new String(b, StandardCharsets.UTF_8);
            out.println("case " + i);
            try { out.println("int " + Integer.parseInt(in)); } catch (NumberFormatException e) { out.println("int error"); }
            try { out.println("long " + Long.parseLong(in)); } catch (NumberFormatException e) { out.println("long error"); }
            out.println("split= " + split(in, "="));
            out.println("split_ " + split(in, " "));
        }
    }
}
