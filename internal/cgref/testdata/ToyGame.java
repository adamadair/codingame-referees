import java.io.*;
import java.util.*;
import java.util.Map.Entry;
import java.util.regex.*;

// A small game for testing the Go port of the referee framework. The test
// appends a framework copy (MultiReferee, SoloReferee, AbstractReferee)
// taken from one of the official referees, compiles it, and compares it
// with the same game written against internal/cgref (toy_test.go).
//
// Rules: every round each player answers "GUESS n" with 0 <= n <= 100. A
// hidden target is drawn from the seeded Random; the closest guesses score a
// point. Guessing out of range kills the player. "QUIT" loses, "WIN" wins
// once the player has 2 points, anything else is invalid input. Reaching
// winScore ends the game. Players also see the guesses made so far this
// round, which shows whether inputs are built per round or per move.
class Referee extends MultiReferee {
    private static final Pattern GUESS = Pattern.compile("GUESS (?<n>-?\\d{1,3})(?: (?<msg>.+))?", Pattern.CASE_INSENSITIVE);

    public static void main(String... args) throws IOException {
        new Referee(System.in, System.out, System.err);
    }

    public Referee(InputStream is, PrintStream out, PrintStream err) throws IOException {
        super(is, out, err);
    }

    private long seed;
    private int rounds, winScore;
    private boolean turnBased;
    private Random random;
    private int playerCount;
    private int[] scores, guesses;
    private boolean[] guessed, dead;
    // Never actually -1 during a game: the game runs inside the super
    // constructor, before this initializer executes. Kept on purpose, since
    // the official referees have the same trap.
    private int lastTarget = -1;

    @Override
    protected void initReferee(int playerCount, Properties prop) throws InvalidFormatException {
        try {
            seed = Long.parseLong(prop.getProperty("seed", String.valueOf(new Random(System.currentTimeMillis()).nextLong())));
            rounds = Integer.parseInt(prop.getProperty("rounds", "30"));
            winScore = Integer.parseInt(prop.getProperty("winScore", "5"));
        } catch (NumberFormatException e) {
            throw new InvalidFormatException("bad number");
        }
        turnBased = Boolean.parseBoolean(prop.getProperty("turnBased", "false"));
        random = new Random(seed);
        this.playerCount = playerCount;
        scores = new int[playerCount];
        guesses = new int[playerCount];
        guessed = new boolean[playerCount];
        dead = new boolean[playerCount];
    }

    @Override
    protected Properties getConfiguration() {
        Properties p = new Properties();
        p.setProperty("seed", String.valueOf(seed));
        p.setProperty("rounds", String.valueOf(rounds));
        p.setProperty("winScore", String.valueOf(winScore));
        p.setProperty("turnBased", String.valueOf(turnBased));
        return p;
    }

    @Override
    protected void populateMessages(Properties p) {
        p.put("quit", "$%d gave up");
        p.put("won", "$%d claimed victory");
        p.put("scoreReached", "$%d reached the target score");
    }

    @Override
    protected boolean isTurnBasedGame() {
        return turnBased;
    }

    @Override
    protected int getMaxRoundCount(int playerCount) {
        return rounds;
    }

    @Override
    protected int getMillisTimeForRound() {
        return 50;
    }

    @Override
    protected String[] getInitDataForView() {
        return new String[] { "toy" };
    }

    @Override
    protected String[] getFrameDataForView(int round, int frame, boolean keyFrame) {
        return new String[] { round + " " + lastTarget };
    }

    @Override
    protected int getExpectedOutputLineCountForPlayer(int playerIdx) {
        return 1;
    }

    @Override
    protected String getGameName() {
        return "Toy";
    }

    @Override
    protected void handlePlayerOutput(int frame, int round, int playerIdx, String[] output)
            throws WinException, LostException, InvalidInputException {
        String line = output[0];
        if (line.equalsIgnoreCase("QUIT")) {
            throw new LostException("quit", playerIdx);
        }
        if (line.equalsIgnoreCase("WIN")) {
            if (scores[playerIdx] >= 2) {
                throw new WinException("won", playerIdx);
            }
            throw new InvalidInputException("GUESS n", line);
        }
        Matcher m = GUESS.matcher(line);
        if (!m.matches()) {
            throw new InvalidInputException("GUESS n", line);
        }
        guesses[playerIdx] = Integer.parseInt(m.group("n"));
        guessed[playerIdx] = true;
    }

    @Override
    protected String[] getInitInputForPlayer(int playerIdx) {
        return new String[] { playerIdx + " " + playerCount + " " + rounds };
    }

    @Override
    protected String[] getInputForPlayer(int round, int playerIdx) {
        StringBuilder state = new StringBuilder();
        StringBuilder seen = new StringBuilder();
        for (int i = 0; i < playerCount; i++) {
            if (i > 0) {
                state.append(' ');
                seen.append(' ');
            }
            state.append(scores[i]).append(':').append(dead[i] ? 1 : 0);
            seen.append(guessed[i] ? String.valueOf(guesses[i]) : "-");
        }
        return new String[] { round + " " + lastTarget, state.toString(), seen.toString() };
    }

    @Override
    protected String getHeadlineAtGameStartForConsole() {
        return null;
    }

    @Override
    protected int getMinimumPlayerCount() {
        return 2;
    }

    @Override
    protected boolean showTooltips() {
        return true;
    }

    @Override
    protected void updateGame(int round) throws GameOverException {
        int target = random.nextInt(100);
        lastTarget = target;
        int best = Integer.MAX_VALUE;
        for (int i = 0; i < playerCount; i++) {
            if (!dead[i] && guessed[i] && guesses[i] >= 0 && guesses[i] <= 100) {
                best = Math.min(best, Math.abs(guesses[i] - target));
            }
        }
        for (int i = 0; i < playerCount; i++) {
            if (!guessed[i] || dead[i]) {
                continue;
            }
            if (guesses[i] < 0 || guesses[i] > 100) {
                dead[i] = true;
            } else if (Math.abs(guesses[i] - target) == best) {
                scores[i]++;
            }
        }
        if (random.nextDouble() < 0.1) {
            scores[random.nextInt(playerCount)] += 1;
        }
        Arrays.fill(guessed, false);
        for (int i = 0; i < playerCount; i++) {
            if (scores[i] >= winScore) {
                throw new GameOverException("scoreReached", i);
            }
        }
    }

    @Override
    protected void prepare(int round) {
    }

    @Override
    protected boolean isPlayerDead(int playerIdx) {
        return dead[playerIdx];
    }

    @Override
    protected String getDeathReason(int playerIdx) {
        return "$" + playerIdx + ": out of range";
    }

    @Override
    protected int getScore(int playerIdx) {
        return scores[playerIdx];
    }

    @Override
    protected String[] getGameSummary(int round) {
        return new String[] { "round " + round };
    }

    @Override
    protected String[] getPlayerActions(int playerIdx, int round) {
        return new String[0];
    }

    @Override
    protected void setPlayerTimeout(int frame, int round, int playerIdx) {
        scores[playerIdx] -= 1;
    }
}

