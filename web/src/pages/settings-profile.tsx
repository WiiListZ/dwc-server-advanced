import { ExternalLink, ImageUp, Trash2 } from "lucide-react"
import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"

import { profileHref, UserAvatar } from "@/components/user-avatar"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { ApiError, api, errorMessage, type MyProfile } from "@/lib/api"

const BIO_MAX = 300
const AVATAR_MAX_BYTES = 2 << 20

async function uploadAvatar(file: File): Promise<MyProfile> {
  const response = await fetch("/api/panel/my/profile/avatar", {
    method: "POST",
    credentials: "same-origin",
    headers: { Accept: "application/json", "Content-Type": file.type || "application/octet-stream" },
    body: file,
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new ApiError(data?.error || `HTTP ${response.status}`, response.status)
  return data as MyProfile
}

/** Public profile settings: avatar, bio and stats visibility. */
export function PublicProfileCard({ onAvatarChange }: { onAvatarChange: (url?: string) => void }) {
  const [profile, setProfile] = useState<MyProfile | null>(null)
  const [bio, setBio] = useState("")
  const [showStats, setShowStats] = useState(true)
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)
  const fileInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    api<MyProfile>("/api/panel/my/profile")
      .then((p) => {
        setProfile(p)
        setBio(p.bio)
        setShowStats(p.show_stats)
      })
      .catch((error) => toast.error(errorMessage(error)))
  }, [])

  function applied(next: MyProfile) {
    setProfile(next)
    onAvatarChange(next.avatar_url)
  }

  async function pickAvatar(file?: File) {
    if (!file) return
    if (file.size > AVATAR_MAX_BYTES) {
      toast.error("Image must be 2 MB or smaller")
      return
    }
    setUploading(true)
    try {
      applied(await uploadAvatar(file))
      toast.success("Avatar updated")
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setUploading(false)
      if (fileInput.current) fileInput.current.value = ""
    }
  }

  async function removeAvatar() {
    setUploading(true)
    try {
      applied(await api<MyProfile>("/api/panel/my/profile/avatar", { method: "DELETE" }))
      toast.success("Avatar removed")
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setUploading(false)
    }
  }

  async function save(event: React.FormEvent) {
    event.preventDefault()
    setSaving(true)
    try {
      const next = await api<MyProfile>("/api/panel/my/profile", { method: "PUT", body: { bio, show_stats: showStats } })
      setProfile(next)
      setBio(next.bio)
      setShowStats(next.show_stats)
      toast.success("Profile saved")
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  if (!profile) {
    return <Skeleton className="h-72 w-full" />
  }

  const dirty = bio !== profile.bio || showStats !== profile.show_stats

  return (
    <Card>
      <CardHeader>
        <CardTitle>Public profile</CardTitle>
        <CardDescription>What everyone sees at {profileHref(profile.username)}.</CardDescription>
        <CardAction>
          <Button asChild variant="outline" size="sm">
            <a href={profileHref(profile.username)}>
              <ExternalLink data-icon="inline-start" /> View
            </a>
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <form onSubmit={save}>
          <FieldGroup>
            <Field>
              <FieldLabel>Avatar</FieldLabel>
              <div className="flex flex-wrap items-center gap-4">
                <UserAvatar name={profile.username} src={profile.avatar_url} className="size-16" fallbackClassName="text-lg" />
                <div className="flex flex-wrap gap-2">
                  <input
                    ref={fileInput}
                    type="file"
                    accept="image/png,image/jpeg,image/gif"
                    className="hidden"
                    onChange={(e) => void pickAvatar(e.target.files?.[0])}
                  />
                  <Button type="button" variant="outline" size="sm" disabled={uploading} onClick={() => fileInput.current?.click()}>
                    {uploading ? <Spinner data-icon="inline-start" /> : <ImageUp data-icon="inline-start" />} Upload image
                  </Button>
                  {profile.avatar_url && (
                    <Button type="button" variant="ghost" size="sm" disabled={uploading} onClick={() => void removeAvatar()}>
                      <Trash2 data-icon="inline-start" /> Remove
                    </Button>
                  )}
                </div>
              </div>
              <FieldDescription>PNG, JPEG or GIF up to 2 MB. It is cropped to a square and resized to 256×256.</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="profile-bio">Bio</FieldLabel>
              <Textarea id="profile-bio" value={bio} maxLength={BIO_MAX} rows={4} placeholder="Tell other players about yourself" onChange={(e) => setBio(e.target.value)} />
              <FieldDescription className="flex justify-between">
                <span>Plain text, shown on your profile.</span>
                <span className="tabular-nums">
                  {[...bio].length}/{BIO_MAX}
                </span>
              </FieldDescription>
            </Field>
            <Field orientation="horizontal" className="justify-between">
              <FieldContent>
                <FieldLabel htmlFor="profile-stats">Show my stats</FieldLabel>
                <FieldDescription>Matches, ratings, time trials, games and friend codes. Your online status is always shown.</FieldDescription>
              </FieldContent>
              <Switch id="profile-stats" checked={showStats} onCheckedChange={setShowStats} />
            </Field>
            <div className="flex justify-end">
              <Button type="submit" disabled={!dirty || saving}>
                {saving && <Spinner data-icon="inline-start" />} Save profile
              </Button>
            </div>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
