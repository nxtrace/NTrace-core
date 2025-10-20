package trace

func ClearCaches() {
	geoCache.Range(func(key, value any) bool {
		geoCache.Delete(key)
		return true
	})
}
