package database

import (
	"dwc-server-advanced/common"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
)

type MarioKartWiiTopTenRanking struct {
	Score      int
	PID        int
	PlayerInfo string
}

const (
	getTopTenRankingsQuery = "" +
		"SELECT score, pid, playerinfo " +
		"FROM mario_kart_wii_sake " +
		"WHERE ($1 = 0 OR regionid = $1) " +
		"AND courseid = $2 " +
		"ORDER BY score ASC " +
		"LIMIT 10"
	getGhostDataQuery = "" +
		"SELECT id " +
		"FROM mario_kart_wii_sake " +
		"WHERE courseid = $1 " +
		"AND score < $2 " +
		"ORDER BY score DESC " +
		"LIMIT 1"
	getStoredGhostDataQuery = "" +
		"SELECT pid, id " +
		"FROM mario_kart_wii_sake " +
		"WHERE ($1 = 0 OR regionid = $1) " +
		"AND courseid = $2 " +
		"ORDER BY score ASC " +
		"LIMIT 1"
	getFileQuery = "" +
		"SELECT ghost " +
		"FROM mario_kart_wii_sake " +
		"WHERE id = $1 " +
		"LIMIT 1"
	getGhostFileQuery = "" +
		"SELECT ghost " +
		"FROM mario_kart_wii_sake " +
		"WHERE courseid = $1 " +
		"AND score < $2 " +
		"AND pid <> $3 " +
		"ORDER BY score DESC " +
		"LIMIT 1"
	insertGhostFileStatement = "" +
		"INSERT INTO mario_kart_wii_sake (regionid, courseid, score, pid, playerinfo, ghost, upload_time) " +
		"VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP) " +
		"ON CONFLICT (courseid, pid) DO UPDATE " +
		"SET regionid = EXCLUDED.regionid, score = EXCLUDED.score, playerinfo = EXCLUDED.playerinfo, ghost = EXCLUDED.ghost, upload_time = CURRENT_TIMESTAMP"
)

func (c *Connection) GetMarioKartWiiTopTenRankings(regionId common.MarioKartWiiLeaderboardRegionId,
	courseId common.MarioKartWiiCourseId) ([]MarioKartWiiTopTenRanking, error) {
	rows, err := c.pool.Query(c.ctx, getTopTenRankingsQuery, regionId, courseId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topTenRankings := make([]MarioKartWiiTopTenRanking, 0, 10)
	for rows.Next() {
		var topTenRanking MarioKartWiiTopTenRanking
		err = rows.Scan(&topTenRanking.Score, &topTenRanking.PID, &topTenRanking.PlayerInfo)
		if err != nil {
			return nil, err
		}

		topTenRankings = append(topTenRankings, topTenRanking)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return topTenRankings, nil
}

func (c *Connection) GetMarioKartWiiGhostData(courseId common.MarioKartWiiCourseId, time int) (int, error) {
	row := c.pool.QueryRow(c.ctx, getGhostDataQuery, courseId, time)

	var fileId int
	if err := row.Scan(&fileId); err != nil {
		return 0, err
	}

	return fileId, nil
}

func (c *Connection) GetMarioKartWiiStoredGhostData(regionId common.MarioKartWiiLeaderboardRegionId,
	courseId common.MarioKartWiiCourseId) (int, int, error) {
	row := c.pool.QueryRow(c.ctx, getStoredGhostDataQuery, regionId, courseId)

	var pid int
	var fileId int
	if err := row.Scan(&pid, &fileId); err != nil {
		return 0, 0, err
	}

	return pid, fileId, nil
}

func (c *Connection) GetMarioKartWiiFile(fileId int) ([]byte, error) {
	row := c.pool.QueryRow(c.ctx, getFileQuery, fileId)

	var file []byte
	if err := row.Scan(&file); err != nil {
		return nil, err
	}

	return file, nil
}

func (c *Connection) GetMarioKartWiiGhostFile(courseId common.MarioKartWiiCourseId, time int, pid int) ([]byte, error) {
	row := c.pool.QueryRow(c.ctx, getGhostFileQuery, courseId, time, pid)

	var ghost []byte
	if err := row.Scan(&ghost); err != nil {
		return nil, err
	}

	return ghost, nil
}

func (c *Connection) InsertMarioKartWiiGhostFile(regionId common.MarioKartWiiLeaderboardRegionId,
	courseId common.MarioKartWiiCourseId, score int, pid int, playerInfo string, ghost []byte) error {
	_, err := c.pool.Exec(c.ctx, insertGhostFileStatement, regionId, courseId, score, pid, playerInfo, ghost)

	return err
}

type MarioKartWiiPanelRanking struct {
	ID         int
	RegionID   int
	CourseID   int
	Score      int
	PID        uint32
	PlayerInfo string
	HasGhost   bool
	GhostSize  int
	UploadTime *time.Time
	Gsbrcd     string
}

const (
	panelRankingSelect = "" +
		"SELECT s.id, s.regionid, s.courseid, s.score, s.pid, s.playerinfo, " +
		"       (s.ghost IS NOT NULL) AS has_ghost, " +
		"       COALESCE(OCTET_LENGTH(s.ghost), 0) AS ghost_size, " +
		"       s.upload_time, COALESCE(u.gsbrcd, '') AS gsbrcd " +
		"FROM public.mario_kart_wii_sake s " +
		"LEFT JOIN public.users u ON u.profile_id = s.pid "

	panelRankingByRegionCourse = panelRankingSelect +
		"WHERE ($1 = 0 OR s.regionid = $1) AND s.courseid = $2 " +
		"ORDER BY s.score ASC LIMIT 10"

	panelRankingByProfile = panelRankingSelect +
		"WHERE s.pid = $1 ORDER BY s.courseid ASC, s.regionid ASC"

	panelGhostByIDQuery = "" +
		"SELECT ghost, courseid, regionid, score, pid, upload_time " +
		"FROM public.mario_kart_wii_sake WHERE id = $1 LIMIT 1"

	panelDeleteRankingByID   = "DELETE FROM public.mario_kart_wii_sake WHERE id = $1"
	panelDeleteRankingsByPID = "DELETE FROM public.mario_kart_wii_sake WHERE pid = $1"
)

func scanPanelRanking(row interface {
	Scan(dest ...any) error
}) (MarioKartWiiPanelRanking, error) {
	var r MarioKartWiiPanelRanking
	var uploadedAt pgtype.Timestamp
	if err := row.Scan(&r.ID, &r.RegionID, &r.CourseID, &r.Score, &r.PID, &r.PlayerInfo, &r.HasGhost, &r.GhostSize, &uploadedAt, &r.Gsbrcd); err != nil {
		return r, err
	}
	if uploadedAt.Status == pgtype.Present {
		t := uploadedAt.Time
		r.UploadTime = &t
	}
	return r, nil
}

func (c *Connection) GetMarioKartWiiPanelTop10(regionId, courseId int) ([]MarioKartWiiPanelRanking, error) {
	rows, err := c.pool.Query(c.ctx, panelRankingByRegionCourse, regionId, courseId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []MarioKartWiiPanelRanking{}
	for rows.Next() {
		r, err := scanPanelRanking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (c *Connection) GetMarioKartWiiPanelRankingsByProfile(pid uint32) ([]MarioKartWiiPanelRanking, error) {
	rows, err := c.pool.Query(c.ctx, panelRankingByProfile, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []MarioKartWiiPanelRanking{}
	for rows.Next() {
		r, err := scanPanelRanking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type MarioKartWiiGhostBlob struct {
	Ghost      []byte
	CourseID   int
	RegionID   int
	Score      int
	PID        uint32
	UploadTime *time.Time
}

func (c *Connection) GetMarioKartWiiPanelGhostByID(id int) (MarioKartWiiGhostBlob, bool, error) {
	var g MarioKartWiiGhostBlob
	var uploadedAt pgtype.Timestamp
	err := c.pool.QueryRow(c.ctx, panelGhostByIDQuery, id).Scan(&g.Ghost, &g.CourseID, &g.RegionID, &g.Score, &g.PID, &uploadedAt)
	if err == pgx.ErrNoRows {
		return g, false, nil
	}
	if err != nil {
		return g, false, err
	}
	if uploadedAt.Status == pgtype.Present {
		t := uploadedAt.Time
		g.UploadTime = &t
	}
	return g, true, nil
}

func (c *Connection) DeleteMarioKartWiiPanelRanking(id int) (int64, error) {
	tag, err := c.pool.Exec(c.ctx, panelDeleteRankingByID, id)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (c *Connection) DeleteMarioKartWiiPanelRankingsByProfile(pid uint32) (int64, error) {
	tag, err := c.pool.Exec(c.ctx, panelDeleteRankingsByPID, pid)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Mario Kart Wii friend info functions for API compatibility

func (c *Connection) GetMKWFriendInfo(profileId uint32) string {
	records, err := c.GetSakeRecords(1687, []int32{int32(profileId)}, "FriendInfo", nil, []string{"info"}, "")
	if err != nil || len(records) == 0 {
		return ""
	}

	infoField, ok := records[0].Fields["info"]
	if !ok {
		return ""
	}

	return infoField.Value
}

func (c *Connection) UpdateMKWFriendInfo(profileId uint32, info string) {
	records, err := c.GetSakeRecords(1687, []int32{int32(profileId)}, "FriendInfo", nil, []string{"info"}, "")
	if err == pgx.ErrNoRows || (err == nil && len(records) == 0) {
		// No existing record, insert new one
		record := SakeRecord{
			GameId:  1687,
			TableId: "FriendInfo",
			OwnerId: int32(profileId),
			Fields: map[string]SakeField{
				"info": {
					Type:  SakeFieldTypeBinaryData,
					Value: info,
				},
			},
		}
		_, err = c.InsertSakeRecord(record)
	} else if err == nil {
		// Update existing record
		records[0].Fields["info"] = SakeField{
			Type:  SakeFieldTypeBinaryData,
			Value: info,
		}
		err = c.UpdateSakeRecord(records[0], int32(profileId))

	}

	if err != nil {
		panic(err)
	}
}
