-- Keep menu/status reads out of players' large intelligence and message blobs.
-- Include every game_players column so SQLite can answer from the index alone.
CREATE INDEX idx_players_game_listing ON players (
  game_id, id, updated_at, user_id, name, num, ready, ai_controlled,
  ai_difficulty, submitted_turn, color, victor, archived, guest
);
