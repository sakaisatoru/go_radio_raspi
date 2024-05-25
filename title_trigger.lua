socket = require("posix.sys.socket")
unistd = require("posix.unistd")

function send_message_mpvradio(message)
    local fd = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM, 0)
    socket.connect(fd, {family = socket.AF_UNIX, path = "/run/mpvradio"})
    socket.send(fd, message)
    unistd.close(fd)
end

function on_media_title_change(name, value)
	if value ~= nil then
		send_message_mpvradio(value)
	end
end
mp.observe_property("metadata/by-key/icy-title", "string", on_media_title_change)
