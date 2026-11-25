import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.*;

// Reads hex-encoded UTF-8 inputs (one per line) from inputs.txt, loads each
// with java.util.Properties and prints the result for the Go tests:
//
//   case <n>
//   <hex key> <hex value>     (sorted by key)
//   error <message>           (if load threw)
public class Gen {
    static String hex(String s) {
        StringBuilder sb = new StringBuilder("x");
        for (byte b : s.getBytes(StandardCharsets.UTF_8)) sb.append(String.format("%02x", b));
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
            Properties p = new Properties();
            try {
                p.load(new StringReader(in));
            } catch (Exception e) {
                out.println("error " + e.getMessage());
            }
            TreeMap<String, String> sorted = new TreeMap<>();
            for (String k : p.stringPropertyNames()) sorted.put(k, p.getProperty(k));
            for (Map.Entry<String, String> e : sorted.entrySet()) out.println(hex(e.getKey()) + " " + hex(e.getValue()));
        }
    }
}
