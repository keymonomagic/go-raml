from sanic import Blueprint
from sanic.views import HTTPMethodView
from sanic.response import text
import drones_api



drones_if = Blueprint('drones_if')


class dronesView(HTTPMethodView):
    
    async def get(self, request):
     
        return await drones_api.drones_get(request)
    
    async def post(self, request):
     
        return await drones_api.drones_post(request)
    
drones_if.add_route(dronesView.as_view(), '/drones')

class drones_bydroneIdView(HTTPMethodView):
    
    async def get(self, request, droneId):
     
        return await drones_api.drones_byDroneId_get(request, droneId)
    
    async def patch(self, request, droneId):
     
        return await drones_api.drones_byDroneId_patch(request, droneId)
    
    async def delete(self, request, droneId):
     
        return await drones_api.drones_byDroneId_delete(request, droneId)
    
drones_if.add_route(drones_bydroneIdView.as_view(), '/drones/<droneId>')

class drones_bydroneId_deliveriesView(HTTPMethodView):
    
    async def get(self, request, droneId):
     
        return await drones_api.drones_byDroneId_deliveries_get(request, droneId)
    
drones_if.add_route(drones_bydroneId_deliveriesView.as_view(), '/drones/<droneId>/deliveries')

