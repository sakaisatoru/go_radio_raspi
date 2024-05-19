require("os")

function on_media_title_change(name, value)
	if value ~= nil then
		os.execute("pkill --signal SIGUSR1 radio")
	end
end
mp.observe_property("media-title", "string", on_media_title_change)
