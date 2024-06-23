-- protocol definition and dissector for overlay message headers
--

overlay_udp_port = 5555

local overlay_proto = Proto("overlay","Overlay Protocol")

local types = {
    [0] = "None",
    [1] = "Handshake",
    [2] = "Data",
    [3] = "Reset",
    [4] = "Rekey",
    [5] = "Close",
    [0xff] = "Punch",
}

--local subtypes = {
--    [0] = "None",
--    [1] = "Protobuf",
--    [2] = "Request",
--    [3] = "Reply",
--    [4] = "Keepalive"
--}
-- protocol fields
local pf_version = ProtoField.new("Version", "overlay.version", ftypes.UINT8)
local pf_type = ProtoField.new("Message Type", "overlay.type", ftypes.UINT8, types)
local pf_index = ProtoField.new("SenderIndex", "overlay.index", ftypes.UINT32)
local pf_counter = ProtoField.new("Message Counter", "overlay.counter", ftypes.UINT64)
local pf_reserved = ProtoField.new("Reserved", "overlay.reserved", ftypes.UINT16, nil, base.HEX)

-- register protofields
overlay_proto.fields = { pf_version, pf_type, pf_index, pf_counter, pf_reserved}

-- field to pull type info for checking payload
local t = Field.new("overlay.type")

local function getTypeValue(typename)
    for k,v in pairs(types) do
        if v == typename then
            return k
        end
    end
    return nil
end

local function isData()
    local typeinfo = t()
    -- get index of "Data" value in table
    local val = getTypeValue("Data")
    if val == typeinfo() then
        return true
    end
    return false
end
-- dissector function
function overlay_proto.dissector(buffer, pinfo, tree)
    pinfo.cols.protocol:set("overlay")
    local pktlen = buffer:reported_length_remaining()

    local root = tree:add(overlay_proto, buffer:range(0, pktlen))

    root:add(pf_version, buffer:range(0, 1))
    root:add(pf_type, buffer:range(1, 1))
    root:add(pf_index, buffer:range(2, 4))
    root:add(pf_counter, buffer:range(6, 8))
    root:add(pf_reserved, buffer:range(14, 2))

    local payload_range = buffer:range(16)
    local payload_tree = root:add("Payload:")
    payload_tree:add("CipherText: " .. payload_range)
    --if isData() then
    --    local ip_dissector = Dissector.get("ip")
    --    ip_dissector:call(buffer(4):tvb(), pinfo, payload_tree)
    --    pinfo.cols.protocol:set("overlay")
    --else
    --    payload_tree:add("Control Message Payload: " .. payload_range)
    --end

    -- local subtree = :add(overlay_proto,buffer(),"overlay Protocol Data")
    -- subtree:add(buffer(0,1), "Version: " .. buffer(0,1))
    -- subtree:add(buffer(1,1), "Type: " .. buffer(1,1))
    -- subtree:add(buffer(2,1), "SubType: " .. buffer(2,1))
    -- subtree:add(buffer(3,1), "Reserved: " .. buffer(3,1))
    -- subtree:add(buffer(4), "Payload: " .. buffer(4))
end

-- load the udp.port table
udp_table = DissectorTable.get("udp.port")
-- register port for overlay protocol
udp_table:add(overlay_udp_port, overlay_proto)